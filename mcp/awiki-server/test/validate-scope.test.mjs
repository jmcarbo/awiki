import { validate } from "../lib/validate-scope.js";
import { readFileSync } from "node:fs";
import { strict as assert } from "node:assert";

const schema = JSON.parse(readFileSync(new URL("../schemas/scope.json", import.meta.url), "utf8"));

// Happy paths
assert.equal(validate(schema, { tag: "memex" }).valid, true);
assert.equal(validate(schema, { slugs: ["s-as-we-may-think"] }).valid, true);
assert.equal(validate(schema, { query: "memex history" }).valid, true);
assert.equal(validate(schema, { tag: "memex", exclude_tags: ["draft"], min_last_updated: "2025-01-01", types: ["source"] }).valid, true);

// oneOf violations
assert.equal(validate(schema, {}).valid, false);
assert.equal(validate(schema, { tag: "memex", slugs: ["x"] }).valid, false);

// Pattern violations
assert.equal(validate(schema, { tag: "-bad" }).valid, false);
assert.equal(validate(schema, { tag: "../etc" }).valid, false);
assert.equal(validate(schema, { slugs: ["-bad"] }).valid, false);

// maxItems / maxLength
assert.equal(validate(schema, { slugs: Array(201).fill("a") }).valid, false);
assert.equal(validate(schema, { tag: "a".repeat(65) }).valid, false);

// additionalProperties
assert.equal(validate(schema, { tag: "memex", evil: "x" }).valid, false);

// types enum
assert.equal(validate(schema, { tag: "memex", types: ["source", "entity"] }).valid, true);
assert.equal(validate(schema, { tag: "memex", types: ["evil"] }).valid, false);

// min_last_updated date pattern
assert.equal(validate(schema, { tag: "memex", min_last_updated: "2025-01-01" }).valid, true);
assert.equal(validate(schema, { tag: "memex", min_last_updated: "yesterday" }).valid, false);

console.log("validate-scope.js: 13/13 ok");
