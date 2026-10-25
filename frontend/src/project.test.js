import assert from "node:assert/strict";
import { test } from "node:test";
import { initialProjectState, previewDomain, submitProject } from "./project.js";

test("domain preview follows the configured pattern and trims inputs", () => {
  assert.equal(previewDomain("<company>-<project>.test", " acme ", " shop "), "acme-shop.test");
  assert.equal(previewDomain("", "acme", "shop"), "shop.acme.local");
  assert.equal(previewDomain("<project>.test", "", "shop"), "");
});

test("create form submits the selected database and import options", async t => {
  let request;
  t.mock.method(globalThis, "fetch", async (url, options) => {
    request = { url, ...options };
    return Response.json({ project: { name: "shop", db: "mysql" }, warnings: ["check hosts"] });
  });
  const data = new FormData();
  for (const [name, value] of Object.entries({
    company: " acme ", project: " shop ", type: "other", php: "8.4", db: "mysql",
    domain: "", import_file: " C:\\dumps\\shop.sql ", import_on_create: "on",
  })) data.set(name, value);
  const state = await submitProject(initialProjectState, data);
  assert.equal(request.url, "/api/projects");
  assert.equal(request.method, "POST");
  assert.deepEqual(JSON.parse(request.body), {
    company: "acme", project: "shop", type: "other", php: "8.4", db: "mysql",
    domain: "", import_file: "C:\\dumps\\shop.sql", import_on_create: true,
  });
  assert.equal(state.error, null);
  assert.equal(state.result.db, "mysql");
  assert.deepEqual(state.warnings, ["check hosts"]);
});

test("create failure keeps the form retryable instead of showing success", async t => {
  t.mock.method(globalThis, "fetch", async () => Response.json({ error: "database unavailable" }, { status: 400 }));
  const data = new FormData();
  data.set("company", "acme");
  data.set("project", "shop");
  const state = await submitProject(initialProjectState, data);
  assert.equal(state.result, null);
  assert.match(state.error, /database unavailable/);
});

test("invalid and duplicate submissions do not call the API", async t => {
  const fetch = t.mock.method(globalThis, "fetch", () => { throw new Error("unexpected fetch"); });
  assert.match((await submitProject(initialProjectState, new FormData())).error, /required/);
  const succeeded = { result: { name: "shop" }, warnings: [], error: null };
  assert.equal(await submitProject(succeeded, new FormData()), succeeded);
  assert.equal(fetch.mock.callCount(), 0);
});
