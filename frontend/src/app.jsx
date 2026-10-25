import {useActionState, useEffect, useMemo, useState} from "react";
import {createRoot} from "react-dom/client";
import "bootstrap/dist/css/bootstrap.min.css";
import {initialProjectState, previewDomain, submitProject} from "./project.js";

function readMeta() {
  try {
    const el = document.getElementById("devhelper-meta");
    if (!el) return { defaultDomainPattern: "<project>.<company>.local" };
    return JSON.parse(el.textContent || "{}");
  } catch {
    return { defaultDomainPattern: "<project>.<company>.local" };
  }
}

function groupByCompany(projects) {
  const m = new Map();
  for (const p of projects || []) {
    const key = (p.company || "").trim() || "unknown";
    if (!m.has(key)) m.set(key, []);
    m.get(key).push(p);
  }
  // stable ordering
  const out = Array.from(m.entries()).sort((a,b) => a[0].localeCompare(b[0]));
  for (const [, arr] of out) {
    arr.sort((x,y) => (x.name || "").localeCompare(y.name || ""));
  }
  return out;
}

async function apiGetProjects() {
  const res = await fetch("/api/projects");
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data.projects || [];
}


async function apiDrop(company, project) {
  const res = await fetch("/api/projects/drop", {
    method: "POST",
    headers: {"Content-Type":"application/json"},
    body: JSON.stringify({company, project}),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

async function apiImport(company, project, file, replace) {
  const res = await fetch("/api/projects/import", {
    method: "POST",
    headers: {"Content-Type":"application/json"},
    body: JSON.stringify({company, project, file, replace}),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

function Badge({children, className=""}) {
  return <span className={"badge text-bg-secondary fw-normal " + className}>{children}</span>;
}

function ProjectRow({p, onRefresh}) {
  const [expanded, setExpanded] = useState(false);
  const [dumpPath, setDumpPath] = useState("");
  const [status, setStatus] = useState(null);
  const [busy, setBusy] = useState(false);

  const domain = p.domain || "";
  const httpUrl = domain ? ("http://" + domain) : "#";
  const httpsUrl = domain ? ("https://" + domain) : "#";

  async function doDrop() {
    if (!confirm(`Drop project ${p.company}/${p.name}? This removes dirs, nginx conf, certs and drops DB.`)) return;
    setBusy(true); setStatus(null);
    try {
      const data = await apiDrop(p.company, p.name);
      if (data.warnings && data.warnings.length) {
        alert("Dropped with warnings:\n\n" + data.warnings.join("\n"));
      }
      await onRefresh();
    } catch (e) {
      alert(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function doImport(replace) {
    if (!dumpPath) {
      setStatus({kind:"warn", msg:"Dump file path is required"});
      return;
    }
    if (replace) {
      if (!confirm(`Replace DB for ${p.company}/${p.name}? This will DROP and recreate the database.`)) return;
    }
    setBusy(true); setStatus(null);
    try {
      await apiImport(p.company, p.name, dumpPath, !!replace);
      setStatus({kind:"ok", msg: replace ? "Replaced + imported." : "Imported."});
    } catch (e) {
      setStatus({kind:"warn", msg:String(e)});
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <tr>
        <td className="align-middle">
          <div className="fw-semibold">{p.name}</div>
          <div className="small text-secondary">{p.type} • PHP {p.php} • DB: {p.db}</div>
        </td>
        <td className="align-middle">
          <div className="d-flex flex-column gap-1">
            <code className="small">{domain}</code>
            <div className="d-flex gap-2 small">
              <a href={httpUrl} target="_blank" rel="noreferrer">http</a>
              <a href={httpsUrl} target="_blank" rel="noreferrer">https</a>
            </div>
          </div>
        </td>
        <td className="align-middle">
          <div className="d-flex gap-2 flex-wrap">
            <button className="btn btn-outline-secondary btn-sm" onClick={() => setExpanded(v => !v)} disabled={busy}>
              {expanded ? "Hide" : "Details"}
            </button>
            <button className="btn btn-outline-danger btn-sm" onClick={doDrop} disabled={busy}>
              Drop
            </button>
          </div>
        </td>
      </tr>

      {expanded && (
        <tr>
          <td colSpan="3">
            <div className="row g-3">
              <div className="col-12 col-lg-6">
                <div className="card">
                  <div className="card-header py-2">
                    <span className="fw-semibold">Credentials & paths</span>
                  </div>
                  <div className="card-body">
                    <pre className="mb-0 small bg-light border rounded p-2">{JSON.stringify({
                      domain: p.domain,
                      php: p.php,
                      db: p.db,
                      db_name: p.db_name,
                      db_user: p.db_user,
                      db_pass: p.db_pass,
                      runtime_path: p.root_path,
                      host_path: p.host_path,
                      nginx_conf: p.nginx_conf,
                    }, null, 2)}</pre>
                  </div>
                </div>
              </div>

              <div className="col-12 col-lg-6">
                <div className="card">
                  <div className="card-header py-2 d-flex justify-content-between align-items-center">
                    <span className="fw-semibold">DB dump (.sql) import</span>
                    <Badge className="text-bg-dark">host path</Badge>
                  </div>
                  <div className="card-body">
                    <div className="mb-2 small text-secondary">
                      Example: <code>E:\dumps\project.sql</code>
                    </div>
                    <div className="input-group">
                      <input className="form-control" value={dumpPath} onChange={e => setDumpPath(e.target.value)} placeholder="E:\dumps\project.sql" />
                      <button className="btn btn-primary" onClick={() => doImport(false)} disabled={busy}>Import</button>
                      <button className="btn btn-outline-primary" onClick={() => doImport(true)} disabled={busy}>Replace</button>
                    </div>

                    {status && (
                      <div className={"mt-2 small " + (status.kind === "ok" ? "text-success" : "text-danger")}>
                        {status.msg}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </div>

          </td>
        </tr>
      )}
    </>
  );
}

function ProjectsView({meta}) {
  const [projects, setProjects] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState(null);

  async function refresh() {
    setLoading(true);
    setErr(null);
    try {
      const p = await apiGetProjects();
      setProjects(p);
    } catch (e) {
      setErr(String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { refresh(); }, []);

  const groups = useMemo(() => groupByCompany(projects), [projects]);

  return (
    <div className="d-flex flex-column gap-3">
      <div className="d-flex align-items-center justify-content-between">
        <div>
          <h1 className="h4 mb-0">Projects</h1>
          <div className="small text-secondary">Grouped by company</div>
        </div>
        <div className="d-flex gap-2">
          <a className="btn btn-warning btn-sm" href="/new">+ New project</a>
          <button className="btn btn-outline-secondary btn-sm" onClick={refresh} disabled={loading}>Refresh</button>
        </div>
      </div>

      {err && <div className="alert alert-danger mb-0">{err}</div>}

      {loading ? (
        <div className="card"><div className="card-body">Loading…</div></div>
      ) : groups.length ? (
        groups.map(([company, list]) => (
          <div className="card" key={company}>
            <div className="card-header d-flex justify-content-between align-items-center">
              <div className="d-flex align-items-center gap-2">
                <span className="fw-semibold">{company}</span>
                <span className="badge text-bg-light border">{list.length}</span>
              </div>
              <div className="small text-secondary">Company</div>
            </div>
            <div className="table-responsive">
              <table className="table table-hover mb-0 align-middle">
                <thead className="table-light">
                  <tr>
                    <th style={{width:"45%"}}>Project</th>
                    <th style={{width:"35%"}}>Domain</th>
                    <th style={{width:"20%"}}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {list.map(p => <ProjectRow key={p.id} p={p} onRefresh={refresh} />)}
                </tbody>
              </table>
            </div>
          </div>
        ))
      ) : (
        <div className="card">
          <div className="card-body">
            No projects yet. <a href="/new">Create one</a>.
          </div>
        </div>
      )}
    </div>
  );
}

function NewProjectView({meta}) {
  const [companies, setCompanies] = useState(["clarivate", "petlink", "rcooler"]);
  const [companyChoice, setCompanyChoice] = useState("");
  const [newCompany, setNewCompany] = useState("");
  const [companiesError, setCompaniesError] = useState(false);
  const [project, setProject] = useState("");
  const [type, setType] = useState("other");
  const [php, setPhp] = useState("8.3");
  const [db, setDb] = useState("none");
  const [domain, setDomain] = useState("");
  const [importFile, setImportFile] = useState("");
  const [importOnCreate, setImportOnCreate] = useState(false);

  const [{result, warnings, error}, submit, busy] = useActionState(submitProject, initialProjectState);

  useEffect(() => {
    let active = true;
    apiGetProjects().then(projects => {
      if (!active) return;
      const existing = projects.map(p => (p.company || "").trim()).filter(Boolean);
      setCompanies(current => [...new Set([...current, ...existing])].sort((a, b) => a.localeCompare(b)));
    }).catch(() => {
      if (active) setCompaniesError(true);
    });
    return () => { active = false; };
  }, []);

  const addingCompany = companyChoice === ":new:";
  const company = addingCompany ? newCompany : companyChoice;
  const defaultDomain = previewDomain(meta.defaultDomainPattern, company, project);

  return (
    <div className="d-flex flex-column gap-3">
      <div className="d-flex align-items-center justify-content-between">
        <div>
          <h1 className="h4 mb-0">New project</h1>
          <div className="small text-secondary">Default domain pattern: <code>{meta.defaultDomainPattern || "<project>.<company>.local"}</code></div>
        </div>
        <div className="d-flex gap-2">
          <a className="btn btn-outline-secondary btn-sm" href="/">← Back</a>
        </div>
      </div>

      {error && <div className="alert alert-danger mb-0">{error}</div>}

      {result ? (
        <div className="card">
          <div className="card-body">
            <div className="mb-2">
              Created. Domain: <code>{result.domain}</code>
            </div>
            {warnings.length ? (
              <div className="alert alert-warning mb-0">
                <div className="fw-semibold">Warnings</div>
                <div className="small">{warnings.join("; ")}</div>
              </div>
            ) : null}
            <div className="mt-3">
              <a className="btn btn-primary btn-sm" href="/">Back to list</a>
            </div>
          </div>
        </div>
      ) : (
        <div className="card">
          <div className="card-body">
            {/* Keep selected engine/runtime after an API error; successful forms unmount. */}
            <form action={submit} onReset={event => event.preventDefault()}>
            <fieldset className="row g-3" disabled={busy}>
              <div className="col-12 col-md-6">
                <label className="form-label" htmlFor="company">Company</label>
                <select className="form-select" id="company" name={addingCompany ? undefined : "company"} required value={companyChoice} onChange={e => setCompanyChoice(e.target.value)}>
                  <option value="" disabled>Select company</option>
                  {companies.map(name => <option key={name} value={name}>{name}</option>)}
                  <option value=":new:">Add new company</option>
                </select>
                {addingCompany && (
                  <div className="mt-2">
                    <label className="form-label" htmlFor="newCompany">New company name</label>
                    <input className="form-control" id="newCompany" name="company" required autoFocus value={newCompany} onChange={e => setNewCompany(e.target.value)} placeholder="company" />
                    <div className="form-text">Added when you create the project.</div>
                  </div>
                )}
                {companiesError && <div className="form-text text-warning">Could not load other companies. You can still add a new company.</div>}
              </div>
              <div className="col-12 col-md-6">
                <label className="form-label">Project</label>
                <input className="form-control" name="project" required value={project} onChange={e => setProject(e.target.value)} placeholder="project" />
              </div>

              <div className="col-12 col-md-4">
                <label className="form-label">Type</label>
                <select className="form-select" name="type" value={type} onChange={e => setType(e.target.value)}>
                  <option value="wordpress">wordpress</option>
                  <option value="laravel">laravel</option>
                  <option value="yii2">yii2</option>
                  <option value="symfony">symfony</option>
                  <option value="other">other</option>
                </select>
              </div>

              <div className="col-12 col-md-4">
                <label className="form-label">PHP</label>
                <select className="form-select" name="php" value={php} onChange={e => setPhp(e.target.value)}>
                  {["7.1","7.2","8.0","8.2","8.3","8.4","8.5","8.6"].map(v => <option key={v} value={v}>{v}</option>)}
                </select>
              </div>

              <div className="col-12 col-md-4">
                <label className="form-label">DB</label>
                <select className="form-select" name="db" value={db} onChange={e => setDb(e.target.value)}>
                  <option value="none">none</option>
                  <option value="mysql">mysql 8.4</option>
                  <option value="mysql9">mysql 9.7</option>
                  <option value="mariadb10">mariadb 10.6</option>
                  <option value="mariadb12">mariadb 12</option>
                  <option value="postgres">postgresql 18</option>
                </select>
              </div>

              <div className="col-12">
                <label className="form-label">Domain (optional)</label>
                <input className="form-control" name="domain" value={domain} onChange={e => setDomain(e.target.value)} placeholder={defaultDomain ? `default: ${defaultDomain}` : "default: <project>.<company>.local"} />
                <div className="form-text">Leave blank to use the default pattern.</div>
              </div>

              <div className="col-12">
                <div className="card border">
                  <div className="card-body">
                    <div className="d-flex justify-content-between align-items-center">
                      <div className="fw-semibold">Optional: import SQL dump after create</div>
                      <Badge className="text-bg-dark">host path</Badge>
                    </div>
                    <div className="mt-2 row g-2 align-items-end">
                      <div className="col-12 col-md-8">
                        <label className="form-label">Dump file (.sql)</label>
                        <input className="form-control" name="import_file" value={importFile} onChange={e => setImportFile(e.target.value)} placeholder="E:\dumps\project.sql" />
                      </div>
                      <div className="col-12 col-md-4">
                        <div className="form-check mt-4">
                          <input className="form-check-input" type="checkbox" name="import_on_create" checked={importOnCreate} onChange={e => setImportOnCreate(e.target.checked)} id="importOnCreate"/>
                          <label className="form-check-label" htmlFor="importOnCreate">Import after create</label>
                        </div>
                      </div>
                    </div>
                    <div className="form-text">Uses root/postgres user for import.</div>
                  </div>
                </div>
              </div>

              <div className="col-12 d-flex gap-2">
                <button className="btn btn-primary" type="submit" disabled={busy}>
                  {busy ? "Creating…" : "Create"}
                </button>
                <a className="btn btn-outline-secondary" href="/">Cancel</a>
              </div>
            </fieldset>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

function App() {
  const meta = useMemo(() => readMeta(), []);
  const isNew = window.location.pathname === "/new";
  return isNew ? <NewProjectView meta={meta}/> : <ProjectsView meta={meta}/>;
}

const root = createRoot(document.getElementById("root"));
root.render(<App/>);
