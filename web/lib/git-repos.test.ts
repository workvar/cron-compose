import assert from "node:assert/strict";
import {
  filterGitRepos,
  gitRepoOwner,
  listGitRepoOwners,
  toggleOwnerFilter,
} from "./git-repos.ts";

assert.equal(gitRepoOwner("workvar/cron-compose"), "workvar");
assert.equal(gitRepoOwner("canaryGrapher/JumpStart"), "canaryGrapher");
assert.equal(gitRepoOwner("group/subgroup/project"), "group/subgroup");
assert.equal(gitRepoOwner("lonely"), "lonely");
assert.equal(gitRepoOwner(""), "");

const repos = [
  { id: "1", full_name: "canaryGrapher/JumpStart", description: "starter", default_branch: "main", clone_url: "", private: false },
  { id: "2", full_name: "workvar/cron-compose", description: "A server management utility tool", default_branch: "main", clone_url: "", private: false },
  { id: "3", full_name: "workvar/peepal-rp", description: "Premier ERP", default_branch: "main", clone_url: "", private: true },
  { id: "4", full_name: "workvar/college-connect", description: "", default_branch: "main", clone_url: "", private: false },
];

assert.deepEqual(listGitRepoOwners(repos, "canaryGrapher"), [
  { owner: "canaryGrapher", personal: true },
  { owner: "workvar", personal: false },
]);
assert.deepEqual(listGitRepoOwners(repos, "nobody"), [
  { owner: "canaryGrapher", personal: false },
  { owner: "workvar", personal: false },
]);
assert.deepEqual(listGitRepoOwners([], "x"), []);

assert.deepEqual(toggleOwnerFilter([], "workvar"), ["workvar"]);
assert.deepEqual(toggleOwnerFilter(["workvar"], "workvar"), []);
assert.deepEqual(toggleOwnerFilter(["workvar"], "canaryGrapher").sort(), ["canaryGrapher", "workvar"]);

{
  const all = filterGitRepos(repos, { query: "", owners: [] });
  assert.equal(all.length, 4);
}
{
  const byOwner = filterGitRepos(repos, { query: "", owners: ["workvar"] });
  assert.deepEqual(byOwner.map((r) => r.id), ["2", "3", "4"]);
}
{
  const multi = filterGitRepos(repos, { query: "", owners: ["workvar", "canaryGrapher"] });
  assert.equal(multi.length, 4);
}
{
  const byText = filterGitRepos(repos, { query: "erp", owners: [] });
  assert.deepEqual(byText.map((r) => r.id), ["3"]);
}
{
  const both = filterGitRepos(repos, { query: "college", owners: ["workvar"] });
  assert.deepEqual(both.map((r) => r.id), ["4"]);
}
{
  const miss = filterGitRepos(repos, { query: "college", owners: ["canaryGrapher"] });
  assert.equal(miss.length, 0);
}
{
  const byName = filterGitRepos(repos, { query: "Jump", owners: [] });
  assert.deepEqual(byName.map((r) => r.id), ["1"]);
}
