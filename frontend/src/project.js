export const defaultDomainPattern = "<project>.<company>.local";

export function previewDomain(pattern, company, project) {
  if (!company.trim() || !project.trim()) return "";
  return (pattern || defaultDomainPattern)
    .replaceAll("<company>", company.trim())
    .replaceAll("<project>", project.trim());
}

export const initialProjectState = { result: null, warnings: [], error: null };

// React 19 form Actions manage pending state and keyboard submission together.
// Return API errors as form state so users can correct input and retry.
export async function submitProject(previousState, formData) {
  // A queued double submission after success must not provision a second time.
  if (previousState.result) return previousState;
  const value = name => String(formData.get(name) || "").trim();
  const body = {
    company: value("company"),
    project: value("project"),
    type: value("type"),
    php: value("php"),
    db: value("db"),
    domain: value("domain"),
    import_file: value("import_file"),
    import_on_create: formData.has("import_on_create"),
  };
  if (!body.company || !body.project) {
    return { ...initialProjectState, error: "Company and Project are required" };
  }
  try {
    const response = await fetch("/api/projects", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || response.statusText);
    return { result: data.project, warnings: data.warnings || [], error: null };
  } catch (error) {
    return { ...initialProjectState, error: String(error) };
  }
}
