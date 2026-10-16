import type { DesignPreview, PreviewDevice, PreviewTheme } from "../api/designs";

export const previewThemes: PreviewTheme[] = ["default", "light", "dark"];
export const previewThemeLabels: Record<PreviewTheme, string> = {
  default: "默认", light: "浅色", dark: "深色",
};

// Keep the chosen theme/device when possible. Theme changes may select the
// other device, but never display an image under the wrong selection label.
export function selectDesignPreview(
  previews: DesignPreview[],
  theme: PreviewTheme = "default",
  device: PreviewDevice = "desktop",
): DesignPreview | undefined {
  const availableTheme = previewThemes.includes(theme) && previews.some((p) => p.theme === theme)
    ? theme
    : previewThemes.find((candidate) => previews.some((p) => p.theme === candidate));
  return previews.find((p) => p.theme === availableTheme && p.device === device)
    ?? previews.find((p) => p.theme === availableTheme && p.device === "desktop")
    ?? previews.find((p) => p.theme === availableTheme);
}
