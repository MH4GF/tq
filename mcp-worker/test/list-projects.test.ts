import { createClient } from "@libsql/client";
import type { Client } from "@libsql/client";
import { beforeEach, describe, expect, it } from "vitest";
import type { ToolContext } from "../src/lib/define-tool";
import listProjects from "../src/mcp/tools/list-projects";

const PROJECTS_DDL = `
CREATE TABLE projects (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  name              TEXT NOT NULL UNIQUE,
  work_dir          TEXT NOT NULL,
  metadata          TEXT NOT NULL DEFAULT '{}',
  dispatch_enabled  INTEGER NOT NULL DEFAULT 1,
  created_at        TEXT NOT NULL DEFAULT (datetime('now'))
)`;

function makeContext(db: Client): ToolContext {
  return { db, props: { login: "MH4GF", accessToken: "test-token" } };
}

async function insertProject(
  db: Client,
  name: string,
  workDir = "",
  dispatchEnabled = 1,
): Promise<void> {
  await db.execute({
    sql: "INSERT INTO projects (name, work_dir, dispatch_enabled) VALUES (?, ?, ?)",
    args: [name, workDir, dispatchEnabled],
  });
}

describe("list_projects", () => {
  let db: Client;

  beforeEach(async () => {
    db = createClient({ url: ":memory:" });
    await db.execute(PROJECTS_DDL);
  });

  it("returns a message when there are no projects", async () => {
    const out = await listProjects.handler({ limit: 20 }, makeContext(db));
    expect(out).toBe("No projects found.");
  });

  it("lists a single project with its fields", async () => {
    await insertProject(db, "alpha", "/work/alpha", 1);
    const out = await listProjects.handler({ limit: 20 }, makeContext(db));
    expect(out).toContain("# tq projects");
    expect(out).toContain("**alpha**");
    expect(out).toContain("work_dir: /work/alpha");
    expect(out).toContain("dispatch: on");
  });

  it("renders empty work_dir as dash and dispatch_enabled 0 as off", async () => {
    await insertProject(db, "bare", "", 0);
    const out = await listProjects.handler({ limit: 20 }, makeContext(db));
    expect(out).toContain("work_dir: -");
    expect(out).toContain("dispatch: off");
  });

  it("orders by id descending and applies limit", async () => {
    await insertProject(db, "first");
    await insertProject(db, "second");
    await insertProject(db, "third");
    const out = await listProjects.handler({ limit: 2 }, makeContext(db));
    const lines = out.split("\n").filter((l) => l.startsWith("- #"));
    expect(lines).toHaveLength(2);
    expect(lines[0]).toContain("**third**");
    expect(lines[1]).toContain("**second**");
    expect(out).not.toContain("**first**");
  });
});
