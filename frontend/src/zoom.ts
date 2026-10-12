// Zoom is the size of the interface in percent, 100 being normal. The Go side
// keeps it in the settings file (Settings.Zoom) and accepts 50 to 200.
export const DEFAULT_ZOOM = 100

// The levels Ctrl+ and Ctrl− step through, as in a browser.
const ZOOM_LEVELS = [50, 67, 75, 80, 90, 100, 110, 125, 150, 175, 200]

export const MIN_ZOOM = ZOOM_LEVELS[0]
export const MAX_ZOOM = ZOOM_LEVELS[ZOOM_LEVELS.length - 1]

// zoomIn and zoomOut give the next level up or down from zoom, which may lie
// between two levels if the settings file was edited by hand. At either end
// they give zoom back.
export function zoomIn(zoom: number): number {
  return ZOOM_LEVELS.find((level) => level > zoom) ?? zoom
}

export function zoomOut(zoom: number): number {
  return [...ZOOM_LEVELS].reverse().find((level) => level < zoom) ?? zoom
}

// applyZoom scales the interface by setting the root font size. Tailwind's
// sizes and spacing are in rem, so the layout grows with the text, while
// viewport heights (h-screen, max-h-[90vh]) still fit the window, which CSS
// zoom would multiply as well.
export function applyZoom(zoom: number) {
  document.documentElement.style.fontSize = zoom === DEFAULT_ZOOM ? '' : `${zoom}%`
}
