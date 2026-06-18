package model

// Artist описывает базовую информацию об исполнителе
type Artist struct {
	ID           int      `json:"id"`
	Image        string   `json:"image"`
	Name         string   `json:"name"`
	Members      []string `json:"members"`
	CreationDate int      `json:"creationDate"`
	FirstAlbum   string   `json:"firstAlbum"` // "DD-MM-YYYY"
}

// Date хранит список дат концертов (как есть в API)
type Date struct {
	ID    int      `json:"id"`
	Dates []string `json:"dates"` // даты могут начинаться со '*'
}

// Location — города, где выступает артист
type Location struct {
	ID        int      `json:"id"`
	Locations []string `json:"locations"`
}

// Relation связывает артиста с картой {город: [даты]}
type Relation struct {
	ID             int                 `json:"id"`
	DatesLocations map[string][]string `json:"datesLocations"`
}
