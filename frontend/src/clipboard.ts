import { ClipboardSetText } from '../wailsjs/runtime/runtime'

// copyText puts text on the clipboard through Wails, or the browser when the
// frontend runs without it (npm run dev).
export async function copyText(text: string): Promise<boolean> {
  try {
    return await ClipboardSetText(text)
  } catch {
    await navigator.clipboard.writeText(text)
    return true
  }
}
