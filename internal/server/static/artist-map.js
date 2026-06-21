"use strict";

const MAP_VIEWBOX_WIDTH = 1000;
const MAP_VIEWBOX_HEIGHT = 500;
const MAP_ASPECT_RATIO = MAP_VIEWBOX_WIDTH / MAP_VIEWBOX_HEIGHT;
const TOUR_MAP_MIN_ZOOM = 1;
const TOUR_MAP_MAX_ZOOM = 3;
const TOUR_MAP_SINGLE_POINT_ZOOM = 3;
const TOUR_MAP_PADDING_RATIO = 0.12;
const TOUR_MAP_MIN_GEO_SPAN_DEGREES = 1;
const TOUR_MAP_WORLD_ZOOM_EPSILON = 0.08;
const TOUR_MAP_GLOBAL_LONGITUDE_SPAN_DEGREES = 120;
const TOUR_MAP_LAND_EDGE_REPEAT_RATIO = 0.20;
const TOUR_MAP_FLOAT_EPSILON = 0.000001;

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function getDefaultTourCamera() {
  return {
    centerX: MAP_VIEWBOX_WIDTH / 2,
    centerY: MAP_VIEWBOX_HEIGHT / 2,
    zoom: TOUR_MAP_MIN_ZOOM,
  };
}

function normalizeViewBoxX(x) {
  const normalized = x % MAP_VIEWBOX_WIDTH;

  return normalized < 0
    ? normalized + MAP_VIEWBOX_WIDTH
    : normalized;
}

function getViewportSize(viewportOrWidth, viewportHeight) {
  if (
    viewportOrWidth &&
    typeof viewportOrWidth === "object"
  ) {
    return {
      width: Number(viewportOrWidth.clientWidth) || 0,
      height: Number(viewportOrWidth.clientHeight) || 0,
    };
  }

  return {
    width: Number(viewportOrWidth) || 0,
    height: Number(viewportHeight) || 0,
  };
}

function projectGeoToViewBox(latitude, longitude) {
  const lat = clamp(Number(latitude), -90, 90);
  const lon = clamp(Number(longitude), -180, 180);

  return {
    x: ((lon + 180) / 360) * MAP_VIEWBOX_WIDTH,
    y: ((90 - lat) / 180) * MAP_VIEWBOX_HEIGHT,
  };
}

function getContainedMapRectForSize(viewportWidth, viewportHeight) {
  const width = Number(viewportWidth) || 0;
  const height = Number(viewportHeight) || 0;

  if (width <= 0 || height <= 0) {
    return { left: 0, top: 0, width: 0, height: 0 };
  }

  const viewportAspect = width / height;

  if (viewportAspect > MAP_ASPECT_RATIO) {
    const containedHeight = height;
    const containedWidth = containedHeight * MAP_ASPECT_RATIO;

    return {
      left: (width - containedWidth) / 2,
      top: 0,
      width: containedWidth,
      height: containedHeight,
    };
  }

  const containedWidth = width;
  const containedHeight = containedWidth / MAP_ASPECT_RATIO;

  return {
    left: 0,
    top: (height - containedHeight) / 2,
    width: containedWidth,
    height: containedHeight,
  };
}

function getContainedMapRect(viewport) {
  const size = getViewportSize(viewport);

  return getContainedMapRectForSize(size.width, size.height);
}

function getCameraZoom(camera) {
  return clamp(
    Number(camera && camera.zoom) || TOUR_MAP_MIN_ZOOM,
    TOUR_MAP_MIN_ZOOM,
    TOUR_MAP_MAX_ZOOM
  );
}

function wrapViewBoxXToCamera(x, centerX) {
  let wrappedX = x;

  while (wrappedX - centerX > MAP_VIEWBOX_WIDTH / 2) {
    wrappedX -= MAP_VIEWBOX_WIDTH;
  }

  while (wrappedX - centerX < -MAP_VIEWBOX_WIDTH / 2) {
    wrappedX += MAP_VIEWBOX_WIDTH;
  }

  return wrappedX;
}

function getLocationCoordinate(location) {
  const source = location && location.coordinate
    ? location.coordinate
    : location;
  const latitude = Number(source && source.latitude);
  const longitude = Number(source && source.longitude);

  if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
    return null;
  }

  return {
    latitude: clamp(latitude, -90, 90),
    longitude: clamp(longitude, -180, 180),
  };
}

function getSmallestCircularViewBoxInterval(points) {
  const sorted = points
    .map(normalizeViewBoxX)
    .sort((a, b) => a - b);

  if (sorted.length === 0) {
    return {
      center: MAP_VIEWBOX_WIDTH / 2,
      span: MAP_VIEWBOX_WIDTH,
    };
  }

  if (sorted.length === 1) {
    return {
      center: sorted[0],
      span: 0,
    };
  }

  let largestGap = -1;
  let largestGapIndex = 0;

  sorted.forEach((point, index) => {
    const next =
      index === sorted.length - 1
        ? sorted[0] + MAP_VIEWBOX_WIDTH
        : sorted[index + 1];
    const gap = next - point;

    if (gap > largestGap) {
      largestGap = gap;
      largestGapIndex = index;
    }
  });

  const startIndex =
    (largestGapIndex + 1) % sorted.length;
  const start = sorted[startIndex];
  let end = sorted[largestGapIndex];

  if (end < start) {
    end += MAP_VIEWBOX_WIDTH;
  }

  const span = end - start;

  return {
    center: normalizeViewBoxX(start + span / 2),
    span,
  };
}

function computeTourCamera(
  locations,
  viewportWidth,
  viewportHeight
) {
  const coordinates = (Array.isArray(locations) ? locations : [])
    .map(getLocationCoordinate)
    .filter(Boolean);
  const viewportSize = getViewportSize(
    viewportWidth,
    viewportHeight
  );
  const mapRect = getContainedMapRectForSize(
    viewportSize.width,
    viewportSize.height
  );

  if (
    coordinates.length === 0 ||
    mapRect.width <= 0 ||
    mapRect.height <= 0
  ) {
    return getDefaultTourCamera();
  }

  if (coordinates.length === 1) {
    const point = projectGeoToViewBox(
      coordinates[0].latitude,
      coordinates[0].longitude
    );

    return {
      centerX: normalizeViewBoxX(point.x),
      centerY: point.y,
      zoom: clamp(
        TOUR_MAP_SINGLE_POINT_ZOOM,
        TOUR_MAP_MIN_ZOOM,
        TOUR_MAP_MAX_ZOOM
      ),
    };
  }

  const projectedPoints = coordinates.map((coordinate) => (
    projectGeoToViewBox(
      coordinate.latitude,
      coordinate.longitude
    )
  ));
  const longitudeInterval = getSmallestCircularViewBoxInterval(
    projectedPoints.map((point) => point.x)
  );
  const latitudeValues = projectedPoints.map((point) => point.y);
  const minY = Math.min(...latitudeValues);
  const maxY = Math.max(...latitudeValues);
  const minLongitudeSpan =
    (TOUR_MAP_MIN_GEO_SPAN_DEGREES / 360) *
    MAP_VIEWBOX_WIDTH;
  const minLatitudeSpan =
    (TOUR_MAP_MIN_GEO_SPAN_DEGREES / 180) *
    MAP_VIEWBOX_HEIGHT;
  const spanX = Math.max(
    longitudeInterval.span,
    minLongitudeSpan
  );
  const spanY = Math.max(maxY - minY, minLatitudeSpan);
  const globalLongitudeSpan =
    (TOUR_MAP_GLOBAL_LONGITUDE_SPAN_DEGREES / 360) *
    MAP_VIEWBOX_WIDTH;

  if (
    longitudeInterval.span + TOUR_MAP_FLOAT_EPSILON >=
    globalLongitudeSpan
  ) {
    return getDefaultTourCamera();
  }

  const availableWidth = viewportSize.width *
    (1 - TOUR_MAP_PADDING_RATIO * 2);
  const availableHeight = viewportSize.height *
    (1 - TOUR_MAP_PADDING_RATIO * 2);
  const mapScale = mapRect.width / MAP_VIEWBOX_WIDTH;
  const zoomX = availableWidth / (spanX * mapScale);
  const zoomY = availableHeight / (spanY * mapScale);
  let zoom = Math.min(zoomX, zoomY);

  if (!Number.isFinite(zoom)) {
    return getDefaultTourCamera();
  }

  zoom = clamp(
    zoom,
    TOUR_MAP_MIN_ZOOM,
    TOUR_MAP_MAX_ZOOM
  );

  if (zoom <= TOUR_MAP_MIN_ZOOM + TOUR_MAP_WORLD_ZOOM_EPSILON) {
    return getDefaultTourCamera();
  }

  return {
    centerX: longitudeInterval.center,
    centerY: (minY + maxY) / 2,
    zoom,
  };
}

function projectGeoWithCamera(
  latitude,
  longitude,
  camera,
  viewportWidth,
  viewportHeight
) {
  const viewBoxPoint = projectGeoToViewBox(latitude, longitude);
  const viewportSize = getViewportSize(
    viewportWidth,
    viewportHeight
  );
  const mapRect = getContainedMapRectForSize(
    viewportSize.width,
    viewportSize.height
  );
  const fallbackCamera = getDefaultTourCamera();
  const centerX = Number.isFinite(Number(camera && camera.centerX))
    ? Number(camera.centerX)
    : fallbackCamera.centerX;
  const centerY = Number.isFinite(Number(camera && camera.centerY))
    ? Number(camera.centerY)
    : fallbackCamera.centerY;
  const zoom = getCameraZoom(camera || fallbackCamera);
  const mapScale = mapRect.width / MAP_VIEWBOX_WIDTH;
  const wrappedX = wrapViewBoxXToCamera(
    viewBoxPoint.x,
    centerX
  );

  return {
    x: viewportSize.width / 2 +
      (wrappedX - centerX) * mapScale * zoom,

    y: viewportSize.height / 2 +
      (viewBoxPoint.y - centerY) * mapScale * zoom,
  };
}

function projectGeoToViewport(latitude, longitude, viewport) {
  return projectGeoWithCamera(
    latitude,
    longitude,
    getDefaultTourCamera(),
    viewport
  );
}

function shouldRepeatLandWorld(camera) {
  const centerX = normalizeViewBoxX(
    Number(camera && camera.centerX) || 0
  );

  return getCameraZoom(camera) > TOUR_MAP_MIN_ZOOM &&
    (
      centerX < MAP_VIEWBOX_WIDTH * TOUR_MAP_LAND_EDGE_REPEAT_RATIO ||
      centerX > MAP_VIEWBOX_WIDTH *
        (1 - TOUR_MAP_LAND_EDGE_REPEAT_RATIO)
    );
}

function applyLandCamera(landLayer, camera, viewport) {
  const viewportSize = getViewportSize(viewport);
  const mapRect = getContainedMapRectForSize(
    viewportSize.width,
    viewportSize.height
  );

  if (
    !landLayer ||
    mapRect.width <= 0 ||
    mapRect.height <= 0
  ) {
    return;
  }

  const repeatWorld = shouldRepeatLandWorld(camera);
  const zoom = getCameraZoom(camera);
  const layerLeft = repeatWorld
    ? mapRect.left - mapRect.width
    : mapRect.left;
  const layerWidth = repeatWorld
    ? mapRect.width * 3
    : mapRect.width;
  const localWorldOffset = repeatWorld
    ? mapRect.width
    : 0;
  const mapScale = mapRect.width / MAP_VIEWBOX_WIDTH;
  const centerX = Number(camera.centerX);
  const centerY = Number(camera.centerY);
  const translateX = viewportSize.width / 2 -
    centerX * mapScale * zoom -
    layerLeft -
    localWorldOffset * zoom;
  const translateY = viewportSize.height / 2 -
    centerY * mapScale * zoom -
    mapRect.top;

  landLayer.classList.toggle("is-world-repeat", repeatWorld);
  landLayer.style.left = `${layerLeft}px`;
  landLayer.style.top = `${mapRect.top}px`;
  landLayer.style.right = "auto";
  landLayer.style.bottom = "auto";
  landLayer.style.width = `${layerWidth}px`;
  landLayer.style.height = `${mapRect.height}px`;
  landLayer.style.transformOrigin = "0 0";
  landLayer.style.transform =
    `matrix(${zoom}, 0, 0, ${zoom}, ` +
    `${translateX}, ${translateY})`;
}

function readLocationCoordinate(card) {
  const latitude = Number(card.dataset.latitude);
  const longitude = Number(card.dataset.longitude);

  if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
    return null;
  }

  return { latitude, longitude };
}

function getMarkerCenter(marker, viewport) {
  const markerRect = marker.getBoundingClientRect();
  const viewportRect = viewport.getBoundingClientRect();

  return {
    x:
      markerRect.left -
      viewportRect.left +
      markerRect.width / 2,

    y:
      markerRect.top -
      viewportRect.top +
      markerRect.height / 2,
  };
}

function getEdgeClearance(point, width, height) {
  return Math.min(
    point.x,
    width - point.x,
    point.y,
    height - point.y
  );
}

function buildFlightPath(start, target, viewport) {
  const dx = target.x - start.x;
  const dy = target.y - start.y;
  const distance = Math.hypot(dx, dy);

  if (distance < 1) {
    return null;
  }

  const midX = (start.x + target.x) / 2;
  const midY = (start.y + target.y) / 2;
  const normalX = -dy / distance;
  const normalY = dx / distance;
  const arcHeight = Math.min(
    120,
    Math.max(36, distance * 0.30)
  );
  const candidateA = {
    x: midX + normalX * arcHeight,
    y: midY + normalY * arcHeight,
  };
  const candidateB = {
    x: midX - normalX * arcHeight,
    y: midY - normalY * arcHeight,
  };
  const width = viewport.clientWidth;
  const height = viewport.clientHeight;
  const selected = getEdgeClearance(candidateA, width, height) >
    getEdgeClearance(candidateB, width, height)
    ? candidateA
    : candidateB;
  const padding = 20;
  const control = {
    x: Math.min(
      width - padding,
      Math.max(padding, selected.x)
    ),

    y: Math.min(
      height - padding,
      Math.max(padding, selected.y)
    ),
  };
  const pathData =
    `M ${start.x} ${start.y} ` +
    `Q ${control.x} ${control.y} ` +
    `${target.x} ${target.y}`;

  return {
    pathData,
    control,
  };
}

function setPlaneAtPathLength(
  routePath,
  plane,
  length,
  totalLength
) {
  const point = routePath.getPointAtLength(length);
  const sampleDistance = Math.max(1, totalLength * 0.003);
  const tangentSample = Math.min(
    totalLength,
    length + sampleDistance
  );
  const nextPoint = routePath.getPointAtLength(tangentSample);
  let tangentX = nextPoint.x - point.x;
  let tangentY = nextPoint.y - point.y;

  if (Math.hypot(tangentX, tangentY) < 0.01) {
    const previousPoint = routePath.getPointAtLength(
      Math.max(0, length - sampleDistance)
    );

    tangentX = point.x - previousPoint.x;
    tangentY = point.y - previousPoint.y;
  }

  const tangentAngle =
    Math.atan2(tangentY, tangentX) *
    (180 / Math.PI);
  const planeAngle = tangentAngle + 90;

  plane.style.left = `${point.x}px`;
  plane.style.top = `${point.y}px`;
  plane.style.transform =
    "translate(-50%, -50%) " +
    `rotate(${planeAngle}deg)`;
}

function easeInOutCubic(value) {
  return value < 0.5
    ? 4 * value * value * value
    : 1 -
        Math.pow(-2 * value + 2, 3) / 2;
}

function easeFlightProgress(value, totalLength) {
  const eased = easeInOutCubic(value);
  const landingShare = Math.min(
    0.60,
    Math.max(0.32, 140 / Math.max(totalLength, 1))
  );
  const landingStart = 1 - landingShare;

  if (value < landingStart) {
    return eased;
  }

  const landingStartValue = easeInOutCubic(landingStart);
  const landingProgress =
    (value - landingStart) /
    (1 - landingStart);
  const landingEase = Math.sin(
    (landingProgress * Math.PI) / 2
  );

  return landingStartValue +
    (1 - landingStartValue) * landingEase;
}

function initArtistMap() {
  const cards = Array.from(
    document.querySelectorAll(".concert-location")
  );

  const viewport = document.querySelector(
    ".tour-map__viewport"
  );

  const markerLayer = document.querySelector(
    ".tour-map__markers"
  );

  const landLayer = document.querySelector(
    ".tour-map__land"
  );

  const routePath = document.querySelector(
    ".tour-map__route"
  );

  const plane = document.querySelector(
    ".tour-map__plane"
  );

  if (
    !cards.length ||
    !viewport ||
    !markerLayer ||
    !landLayer ||
    !routePath ||
    !plane
  ) {
    return;
  }

  const reducedMotionQuery = window.matchMedia(
    "(prefers-reduced-motion: reduce)"
  );

  let currentIndex = 0;
  let targetIndex = null;
  let isFlying = false;
  let animationFrameId = null;
  let lastRoute = null;
  let camera = getDefaultTourCamera();

  const locations = cards.map((card, index) => {
    const locationIndex = Number(card.dataset.locationIndex || index);
    const coordinate = readLocationCoordinate(card);

    if (!coordinate) {
      return null;
    }

    return {
      card,
      locationIndex,
      coordinate,
    };
  });

  if (locations.some((location) => location === null)) {
    console.error("Tour map location is missing coordinates.");
    return;
  }

  markerLayer.replaceChildren();

  const markers = locations.map((location) => {
    const marker = document.createElement("span");

    marker.className = "tour-map__marker";
    marker.dataset.locationIndex = String(location.locationIndex);
    marker.dataset.latitude = String(location.coordinate.latitude);
    marker.dataset.longitude = String(location.coordinate.longitude);

    if (location.locationIndex === 0) {
      marker.classList.add("is-active");
    }

    location.card.classList.toggle(
      "is-active",
      location.locationIndex === 0
    );
    location.card.setAttribute(
      "aria-pressed",
      location.locationIndex === 0 ? "true" : "false"
    );

    markerLayer.append(marker);

    return marker;
  });

  function getMarker(index) {
    return markers.find((marker) => (
      Number(marker.dataset.locationIndex) === index
    ));
  }

  function updateCamera() {
    camera = computeTourCamera(
      locations.map((location) => location.coordinate),
      viewport
    );

    applyLandCamera(landLayer, camera, viewport);
  }

  function placeMarkers() {
    markers.forEach((marker) => {
      const position = projectGeoWithCamera(
        marker.dataset.latitude,
        marker.dataset.longitude,
        camera,
        viewport
      );

      marker.style.left = `${position.x}px`;
      marker.style.top = `${position.y}px`;
    });
  }

  function getCard(index) {
    return cards.find((card) => (
      Number(card.dataset.locationIndex) === index
    ));
  }

  function clearRoute() {
    routePath.removeAttribute("d");
    routePath.classList.remove("is-visible", "is-flying");
  }

  function setActiveState(index) {
    cards.forEach((card) => {
      const isActive = Number(card.dataset.locationIndex) === index;

      card.classList.toggle("is-active", isActive);
      card.setAttribute("aria-pressed", isActive ? "true" : "false");
    });

    markers.forEach((marker) => {
      marker.classList.toggle(
        "is-active",
        Number(marker.dataset.locationIndex) === index
      );
    });
  }

  function setPlaneRestingRotation() {
    plane.style.transform =
      "translate(-50%, -50%) rotate(90deg)";
  }

  function placePlaneAtMarker(index) {
    const marker = getMarker(index);

    if (!marker) {
      return;
    }

    const point = getMarkerCenter(marker, viewport);

    plane.style.left = `${point.x}px`;
    plane.style.top = `${point.y}px`;

    if (!lastRoute) {
      setPlaneRestingRotation();
    }
  }

  function buildRouteBetween(fromIndex, toIndex) {
    const currentMarker = getMarker(fromIndex);
    const targetMarker = getMarker(toIndex);

    if (!currentMarker || !targetMarker) {
      return null;
    }

    const start = getMarkerCenter(currentMarker, viewport);
    const target = getMarkerCenter(targetMarker, viewport);
    const flightPath = buildFlightPath(start, target, viewport);

    if (!flightPath) {
      return null;
    }

    routePath.setAttribute("d", flightPath.pathData);

    return {
      fromIndex,
      toIndex,
      totalLength: routePath.getTotalLength(),
    };
  }

  function completeSelectionWithoutRoute(selectedIndex) {
    setActiveState(selectedIndex);
    clearRoute();
    lastRoute = null;
    currentIndex = selectedIndex;
    targetIndex = null;
    isFlying = false;
    plane.classList.remove("is-flying");
    plane.dataset.currentLocationIndex = String(selectedIndex);
    placePlaneAtMarker(selectedIndex);
  }

  function landAtRouteEnd(route) {
    setPlaneAtPathLength(
      routePath,
      plane,
      route.totalLength,
      route.totalLength
    );

    plane.classList.remove("is-flying");
    routePath.classList.remove("is-flying");
    routePath.classList.add("is-visible");
    currentIndex = route.toIndex;
    targetIndex = null;
    isFlying = false;
    plane.dataset.currentLocationIndex = String(route.toIndex);
    lastRoute = {
      fromIndex: route.fromIndex,
      toIndex: route.toIndex,
    };
  }

  function animateFlight({
    routePath,
    plane,
    duration,
    onComplete,
  }) {
    const totalLength = routePath.getTotalLength();
    let startedAt = null;
    let isComplete = false;

    function complete() {
      if (isComplete) {
        return;
      }

      isComplete = true;
      animationFrameId = null;
      setPlaneAtPathLength(
        routePath,
        plane,
        totalLength,
        totalLength
      );
      onComplete();
    }

    function tick(timestamp) {
      if (startedAt === null) {
        startedAt = timestamp;
      }

      const elapsed = timestamp - startedAt;
      const rawProgress = Math.min(
        elapsed / duration,
        1
      );
      const progress = easeFlightProgress(rawProgress, totalLength);
      const currentLength = totalLength * progress;

      setPlaneAtPathLength(
        routePath,
        plane,
        currentLength,
        totalLength
      );

      if (rawProgress >= 1) {
        complete();
        return;
      }

      animationFrameId = window.requestAnimationFrame(tick);
    }

    animationFrameId = window.requestAnimationFrame(tick);
  }

  function completeActiveFlightImmediately() {
    if (!isFlying || targetIndex === null) {
      return;
    }

    const route = buildRouteBetween(currentIndex, targetIndex);

    if (animationFrameId !== null) {
      window.cancelAnimationFrame(animationFrameId);
      animationFrameId = null;
    }

    if (!route) {
      completeSelectionWithoutRoute(targetIndex);
      return;
    }

    setActiveState(targetIndex);
    routePath.classList.add("is-visible");
    routePath.classList.remove("is-flying");
    landAtRouteEnd(route);
  }

  function selectLocation(selectedIndex) {
    if (
      isFlying ||
      selectedIndex === currentIndex ||
      !getCard(selectedIndex) ||
      !getMarker(selectedIndex)
    ) {
      return;
    }

    const fromIndex = currentIndex;
    const route = buildRouteBetween(fromIndex, selectedIndex);

    if (!route) {
      completeSelectionWithoutRoute(selectedIndex);
      return;
    }

    routePath.classList.add("is-visible");
    setActiveState(selectedIndex);
    setPlaneAtPathLength(routePath, plane, 0, route.totalLength);

    if (reducedMotionQuery.matches) {
      routePath.classList.remove("is-flying");
      landAtRouteEnd(route);
      return;
    }

    const duration = Math.round(
      Math.min(
        2000,
        Math.max(1100, route.totalLength * 4.8)
      )
    );

    isFlying = true;
    targetIndex = selectedIndex;
    routePath.classList.add("is-flying");
    plane.classList.add("is-flying");

    animateFlight({
      routePath,
      plane,
      duration,
      onComplete: () => {
        landAtRouteEnd(route);
      },
    });
  }

  cards.forEach((card) => {
    card.addEventListener("click", () => {
      selectLocation(Number(card.dataset.locationIndex));
    });

    card.addEventListener("keydown", (event) => {
      if (event.key !== "Enter" && event.key !== " ") {
        return;
      }

      if (event.key === " ") {
        event.preventDefault();
      }

      selectLocation(Number(card.dataset.locationIndex));
    });
  });

  const resizeObserver = new ResizeObserver(() => {
    updateCamera();
    placeMarkers();

    if (isFlying) {
      completeActiveFlightImmediately();
      return;
    }

    if (lastRoute) {
      const route = buildRouteBetween(
        lastRoute.fromIndex,
        lastRoute.toIndex
      );

      if (route) {
        routePath.classList.add("is-visible");
        routePath.classList.remove("is-flying");
        setPlaneAtPathLength(
          routePath,
          plane,
          route.totalLength,
          route.totalLength
        );
        return;
      }
    }

    placePlaneAtMarker(currentIndex);
  });

  updateCamera();
  placeMarkers();
  placePlaneAtMarker(0);
  plane.dataset.currentLocationIndex = "0";
  resizeObserver.observe(viewport);
}

if (typeof document !== "undefined") {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initArtistMap);
  } else {
    initArtistMap();
  }
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    TOUR_MAP_MAX_ZOOM,
    TOUR_MAP_MIN_ZOOM,
    TOUR_MAP_PADDING_RATIO,
    TOUR_MAP_SINGLE_POINT_ZOOM,
    computeTourCamera,
    getContainedMapRect,
    getContainedMapRectForSize,
    projectGeoToViewBox,
    projectGeoToViewport,
    projectGeoWithCamera,
  };
}
