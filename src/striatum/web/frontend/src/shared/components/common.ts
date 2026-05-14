export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

export function labelize(value: string): string {
  return value.replace(/_/g, " ");
}

export function canonicalizeByline(line: string | null | undefined): string | null {
  if (!line) return null;
  let stripped = line.trimStart();
  while (stripped.startsWith("#")) {
    stripped = stripped.slice(1);
  }
  stripped = stripped.trim().replace(/\*/g, "").replace(/_/g, "");
  if (!stripped.toLowerCase().startsWith("author:")) return null;
  const suffix = stripped.split(":", 2)[1]?.trim() ?? "";
  return `author: ${suffix.toLowerCase()}`;
}
