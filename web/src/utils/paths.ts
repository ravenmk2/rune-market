export function skillPublishVersionPath(namespace: string, name: string): string {
  return `/publish/skills/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`;
}
