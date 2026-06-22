package geocode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"groupie-tracker/internal/catalog"
)

const (
	DefaultNominatimURL       = "https://nominatim.openstreetmap.org/search"
	DefaultNominatimUserAgent = "groupie-tracker-geolocalization/1.0 (student project; local repository)"
	defaultNominatimTimeout   = 8 * time.Second
	defaultNominatimInterval  = time.Second
)

var (
	ErrNoResults      = errors.New("nominatim returned no results")
	ErrNoMatch        = errors.New("nominatim returned no acceptable match")
	ErrAmbiguousMatch = errors.New("nominatim returned an ambiguous match")
)

type MatchMethod string

const (
	MatchExact MatchMethod = "exact"
	MatchFuzzy MatchMethod = "fuzzy"
)

type Match struct {
	Coordinate
	Method MatchMethod
}

type NominatimClientConfig struct {
	BaseURL     string
	HTTPClient  *http.Client
	UserAgent   string
	MinInterval time.Duration
	Logger      *log.Logger
}

type NominatimClient struct {
	baseURL     string
	httpClient  *http.Client
	userAgent   string
	minInterval time.Duration
	logger      *log.Logger

	mu          sync.Mutex
	lastRequest time.Time
}

type NominatimResult struct {
	Lat         string           `json:"lat"`
	Lon         string           `json:"lon"`
	Name        string           `json:"name"`
	DisplayName string           `json:"display_name"`
	Address     nominatimAddress `json:"address"`
}

type nominatimAddress struct {
	City         string `json:"city"`
	CityDistrict string `json:"city_district"`
	Town         string `json:"town"`
	Village      string `json:"village"`
	Municipality string `json:"municipality"`
	Hamlet       string `json:"hamlet"`
	Suburb       string `json:"suburb"`
	County       string `json:"county"`
	Province     string `json:"province"`
	State        string `json:"state"`
	Region       string `json:"region"`
	Archipelago  string `json:"archipelago"`
	Country      string `json:"country"`
	CountryCode  string `json:"country_code"`
}

func NewNominatimClient(config NominatimClientConfig) *NominatimClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultNominatimURL
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultNominatimTimeout}
	}
	userAgent := config.UserAgent
	if userAgent == "" {
		userAgent = DefaultNominatimUserAgent
	}
	minInterval := config.MinInterval
	if minInterval == 0 {
		minInterval = defaultNominatimInterval
	}
	logger := config.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &NominatimClient{
		baseURL:     baseURL,
		httpClient:  httpClient,
		userAgent:   userAgent,
		minInterval: minInterval,
		logger:      logger,
	}
}

func (c *NominatimClient) Geocode(ctx context.Context, spec LocationSpec) (Match, error) {
	if c == nil {
		return Match{}, fmt.Errorf("nominatim client is nil")
	}
	if err := ctx.Err(); err != nil {
		return Match{}, err
	}
	results, err := c.search(ctx, spec.Query)
	if err != nil {
		return Match{}, err
	}
	match, err := SelectExactMatch(spec, results)
	if err == nil {
		c.logger.Printf(
			"geocoding matched %q by %s at %.6f, %.6f",
			spec.Display,
			match.Method,
			match.Latitude,
			match.Longitude,
		)
		return match, nil
	}
	if !errors.Is(err, ErrNoMatch) {
		c.logger.Printf("geocoding rejected exact result for %q: %v", spec.Display, err)
		return Match{}, err
	}

	reason := "primary search returned no results"
	if len(results) > 0 {
		reason = "primary search returned candidates without an exact match"
	}
	c.logger.Printf("geocoding fallback for %q: %s", spec.Display, reason)

	if err := ctx.Err(); err != nil {
		return Match{}, err
	}
	fallbackResults, fallbackQuery, err := c.searchFallback(ctx, spec)
	if err != nil {
		return Match{}, err
	}
	c.logger.Printf("geocoding fallback query for %q: %s", spec.Display, fallbackQuery)
	if len(fallbackResults) == 0 {
		c.logger.Printf("geocoding fallback no-match for %q: no candidates", spec.Display)
		return Match{}, ErrNoResults
	}
	match, err = SelectFuzzyMatch(spec, fallbackResults)
	if err != nil {
		if errors.Is(err, ErrAmbiguousMatch) {
			c.logger.Printf("geocoding fallback ambiguous for %q", spec.Display)
		} else {
			c.logger.Printf("geocoding fallback rejected for %q: %v", spec.Display, err)
		}
		return Match{}, err
	}
	c.logger.Printf(
		"geocoding fallback accepted %q at %.6f, %.6f",
		spec.Display,
		match.Latitude,
		match.Longitude,
	)
	c.logger.Printf(
		"geocoding matched %q by %s at %.6f, %.6f",
		spec.Display,
		match.Method,
		match.Latitude,
		match.Longitude,
	)
	return match, nil
}

func (c *NominatimClient) search(ctx context.Context, query string) ([]NominatimResult, error) {
	return c.searchValues(ctx, query, nil)
}

func (c *NominatimClient) searchFallback(ctx context.Context, spec LocationSpec) ([]NominatimResult, string, error) {
	city := normalizedFallbackCity(spec.City)
	country := strings.TrimSpace(spec.Country)
	values := url.Values{}
	values.Set("city", city)
	values.Set("country", country)
	queryForLog := fmt.Sprintf("city=%q country=%q", city, country)
	results, err := c.searchValues(ctx, "", values)
	return results, queryForLog, err
}

func (c *NominatimClient) searchValues(ctx context.Context, query string, structured url.Values) ([]NominatimResult, error) {
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("format", "jsonv2")
	values.Set("addressdetails", "1")
	values.Set("accept-language", "en")
	values.Set("limit", "5")
	if query != "" {
		values.Set("q", query)
	}
	for key, items := range structured {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	searchURL := c.baseURL + "?" + values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build nominatim request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Printf("geocoding HTTP error for %q: %v", query, err)
		return nil, fmt.Errorf("nominatim request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		c.logger.Printf("geocoding HTTP status %d for %q: %s", resp.StatusCode, query, strings.TrimSpace(string(body)))
		return nil, fmt.Errorf("nominatim HTTP status %d", resp.StatusCode)
	}

	var results []NominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		c.logger.Printf("geocoding decode error for %q: %v", query, err)
		return nil, fmt.Errorf("decode nominatim response: %w", err)
	}
	return results, nil
}

func normalizedFallbackCity(city string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(city, "_", " ")), " ")
}

func (c *NominatimClient) waitForRateLimit(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.lastRequest.IsZero() && c.minInterval > 0 {
		wait := c.minInterval - time.Since(c.lastRequest)
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
	}
	c.lastRequest = time.Now()
	return nil
}

func SelectMatch(spec LocationSpec, results []NominatimResult) (Match, error) {
	if match, err := SelectExactMatch(spec, results); err == nil {
		return match, nil
	} else if !errors.Is(err, ErrNoMatch) {
		return Match{}, err
	}
	return SelectFuzzyMatch(spec, results)
}

func SelectExactMatch(spec LocationSpec, results []NominatimResult) (Match, error) {
	for _, result := range results {
		if !matchesExact(spec, result) {
			continue
		}
		coordinate, err := coordinateFromResult(result)
		if err != nil {
			return Match{}, err
		}
		return Match{Coordinate: coordinate, Method: MatchExact}, nil
	}
	return Match{}, ErrNoMatch
}

func SelectFuzzyMatch(spec LocationSpec, results []NominatimResult) (Match, error) {
	var best Match
	bestScore := maxInt()
	ambiguous := false
	for _, result := range results {
		score, ok := fuzzyMatchScore(spec, result)
		if !ok {
			continue
		}
		coordinate, err := coordinateFromResult(result)
		if err != nil {
			return Match{}, err
		}
		if score < bestScore {
			bestScore = score
			best = Match{Coordinate: coordinate, Method: MatchFuzzy}
			ambiguous = false
			continue
		}
		if score == bestScore && !sameCoordinate(best.Coordinate, coordinate) {
			ambiguous = true
		}
	}
	if ambiguous {
		return Match{}, ErrAmbiguousMatch
	}
	if bestScore != maxInt() {
		return best, nil
	}
	return Match{}, ErrNoMatch
}

func matchesExact(spec LocationSpec, result NominatimResult) bool {
	return exactAny(spec.City, candidateCities(result)) &&
		exactCountry(spec, candidateCountries(result))
}

func fuzzyMatchScore(spec LocationSpec, result NominatimResult) (int, bool) {
	cityScore, ok := bestTextScore(spec.City, candidateCities(result))
	if !ok {
		return 0, false
	}
	if !exactCountry(spec, candidateCountries(result)) {
		return 0, false
	}
	return cityScore, true
}

func bestTextScore(expected string, candidates []string) (int, bool) {
	expectedNorm := normalizeMatchText(expected)
	if expectedNorm == "" {
		return 0, false
	}
	best := maxInt()
	for _, candidate := range candidates {
		candidateNorm := normalizeMatchText(candidate)
		if candidateNorm == "" {
			continue
		}
		if candidateNorm == expectedNorm {
			return 0, true
		}
		if distance, ok := catalog.SearchTextFuzzyDistance(expected, candidate); ok && distance < best {
			best = distance
		}
	}
	if best == maxInt() {
		return 0, false
	}
	return best, true
}

func exactAny(expected string, candidates []string) bool {
	expectedNorm := normalizeMatchText(expected)
	if expectedNorm == "" {
		return false
	}
	for _, candidate := range candidates {
		if normalizeMatchText(candidate) == expectedNorm {
			return true
		}
	}
	return false
}

func exactCountry(spec LocationSpec, candidates []string) bool {
	aliases := spec.CountryAliases
	if len(aliases) == 0 {
		aliases = []string{spec.Country}
	}
	aliasSet := make(map[string]struct{}, len(aliases))
	for _, alias := range aliases {
		if normalized := normalizeMatchText(alias); normalized != "" {
			aliasSet[normalized] = struct{}{}
		}
	}
	for _, candidate := range candidates {
		normalized := normalizeMatchText(candidate)
		if _, ok := aliasSet[normalized]; ok {
			return true
		}
	}
	return false
}

func candidateCities(result NominatimResult) []string {
	values := []string{
		result.Name,
		result.Address.City,
		result.Address.CityDistrict,
		result.Address.Town,
		result.Address.Village,
		result.Address.Municipality,
		result.Address.Hamlet,
		result.Address.Suburb,
	}
	if first := firstDisplayComponent(result.DisplayName); first != "" {
		values = append(values, first)
	}
	expanded := make([]string, 0, len(values)*2)
	for _, value := range values {
		expanded = append(expanded, cityNameVariants(value)...)
	}
	return nonEmptyUnique(expanded)
}

func candidateCountries(result NominatimResult) []string {
	values := []string{
		result.Address.Country,
		result.Address.CountryCode,
	}
	if last := displayCountryComponent(result); last != "" {
		values = append(values, last)
	}
	return nonEmptyUnique(values)
}

func displayCountryComponent(result NominatimResult) string {
	last := lastDisplayComponent(result.DisplayName)
	if last == "" {
		return ""
	}
	for _, subdivision := range []string{
		result.Address.State,
		result.Address.Province,
		result.Address.Region,
		result.Address.Archipelago,
	} {
		if normalizeMatchText(last) != "" && normalizeMatchText(last) == normalizeMatchText(subdivision) {
			return ""
		}
	}
	return last
}

func firstDisplayComponent(display string) string {
	parts := strings.Split(display, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func lastDisplayComponent(display string) string {
	parts := strings.Split(display, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		if value := strings.TrimSpace(parts[i]); value != "" {
			return value
		}
	}
	return ""
}

func nonEmptyUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized := normalizeMatchText(trimmed)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func cityNameVariants(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	variants := []string{trimmed}
	lower := strings.ToLower(trimmed)
	prefixes := []string{
		"city of ",
		"greater ",
		"municipality of ",
		"metropolitan city of ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			variants = append(variants, strings.TrimSpace(trimmed[len(prefix):]))
		}
	}
	suffixes := []string{
		" city",
		" municipality",
		" metropolitan area",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) && len(trimmed) > len(suffix) {
			variants = append(variants, strings.TrimSpace(trimmed[:len(trimmed)-len(suffix)]))
		}
	}
	fields := strings.Fields(normalizeMatchText(trimmed))
	if len(fields) > 1 && fields[0] == "saint" {
		variants = append(variants, "st "+strings.Join(fields[1:], " "))
	}
	if len(fields) > 1 && fields[0] == "st" {
		variants = append(variants, "saint "+strings.Join(fields[1:], " "))
	}
	return variants
}

func coordinateFromResult(result NominatimResult) (Coordinate, error) {
	latitude, err := strconv.ParseFloat(result.Lat, 64)
	if err != nil {
		return Coordinate{}, fmt.Errorf("parse nominatim latitude %q: %w", result.Lat, err)
	}
	longitude, err := strconv.ParseFloat(result.Lon, 64)
	if err != nil {
		return Coordinate{}, fmt.Errorf("parse nominatim longitude %q: %w", result.Lon, err)
	}
	if !validLatitude(latitude) || !validLongitude(longitude) {
		return Coordinate{}, fmt.Errorf("nominatim coordinates out of bounds: latitude=%f longitude=%f", latitude, longitude)
	}
	return Coordinate{Latitude: latitude, Longitude: longitude}, nil
}

func sameCoordinate(left Coordinate, right Coordinate) bool {
	return math.Abs(left.Latitude-right.Latitude) < 0.000001 &&
		math.Abs(left.Longitude-right.Longitude) < 0.000001
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
