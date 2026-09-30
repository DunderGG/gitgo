import type { Config } from 'tailwindcss'
import colors from 'tailwindcss/colors'
import plugin from 'tailwindcss/plugin'

// The UI is written with dark-theme classes (bg-gray-900, text-gray-100, …).
// Rather than adding a light class next to every one, the palettes it uses
// are CSS variables: the dark theme gets Tailwind's own colours, and the light
// theme (<html data-theme="light">, set by src/theme.ts) swaps each shade for
// the one that plays the same part on a light background. Grays and tints
// swap ends of the scale; the accent shades used for buttons and selection
// borders (500–700) stay as they are.
const shades = ['50', '100', '200', '300', '400', '500', '600', '700', '800', '900', '950'] as const
type Shade = (typeof shades)[number]
type Palette = 'gray' | 'indigo' | 'yellow' | 'red' | 'sky'

const accentLight: Record<Shade, Shade> = {
  50: '950',
  100: '900',
  200: '800',
  300: '700',
  400: '600',
  500: '500',
  600: '600',
  700: '700',
  800: '200',
  900: '300',
  950: '100',
}

// Light colour for each dark shade: a shade of the same palette or a hex value.
const lightShades: Record<Palette, Record<Shade, string>> = {
  gray: {
    50: '950',
    100: '900',
    200: '800',
    300: '700',
    400: '600',
    500: '500',
    600: '400',
    700: '200',
    800: '100',
    900: '50',
    950: '#ffffff',
  },
  // Unpushed dots and selection: indigo-400 would be faint on white.
  indigo: { ...accentLight, 400: '500' },
  yellow: accentLight,
  red: accentLight,
  sky: accentLight,
}

const palettes = Object.keys(lightShades) as Palette[]

// '#rrggbb' → 'r g b', the form rgb(var(--x) / <alpha-value>) needs.
function channels(hex: string): string {
  const value = parseInt(hex.slice(1), 16)
  return `${(value >> 16) & 255} ${(value >> 8) & 255} ${value & 255}`
}

function variables(pick: (palette: Palette, shade: Shade) => string): Record<string, string> {
  const result: Record<string, string> = {}
  for (const palette of palettes) {
    for (const shade of shades) {
      result[`--color-${palette}-${shade}`] = channels(pick(palette, shade))
    }
  }
  return result
}

const darkVariables = variables((palette, shade) => colors[palette][shade])
const lightVariables = variables((palette, shade) => {
  const light = lightShades[palette][shade]
  return light.startsWith('#') ? light : colors[palette][light as Shade]
})

const config: Config = {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ...Object.fromEntries(
          palettes.map((palette) => [
            palette,
            Object.fromEntries(shades.map((shade) => [shade, `rgb(var(--color-${palette}-${shade}) / <alpha-value>)`])),
          ]),
        ),
        // Behind dialogs: dimmed in both themes.
        scrim: 'rgb(var(--color-scrim))',
      },
    },
  },
  plugins: [
    plugin(({ addBase }) => {
      addBase({
        ':root': { ...darkVariables, '--color-scrim': '3 7 18 / 0.75', 'color-scheme': 'dark' },
        ':root[data-theme="light"]': { ...lightVariables, '--color-scrim': '17 24 39 / 0.4', 'color-scheme': 'light' },
      })
    }),
  ],
}

export default config
