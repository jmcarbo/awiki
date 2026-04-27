// Tiny JSON Schema validator. Handles only the keywords used by scope.json:
// type, additionalProperties, oneOf, required, properties, pattern, maxLength,
// minItems, maxItems, enum, items. Returns {valid: boolean, errors: string[]}.
//
// Hand-rolled to stay consistent with phase-08's no-zod, no-ajv posture.
// Keeps surface area small; adding new keywords requires a deliberate edit
// here (and a corresponding test in test/validate-scope.test.mjs).

export function validate(schema, data, path = "") {
  const errors = [];

  if (schema.type) {
    const expected = schema.type;
    const actual = Array.isArray(data) ? "array" : data === null ? "null" : typeof data;
    if (actual !== expected) {
      errors.push(`${path || "(root)"}: expected ${expected}, got ${actual}`);
      return { valid: false, errors };
    }
  }

  if (schema.enum) {
    if (!schema.enum.includes(data)) {
      errors.push(`${path}: value ${JSON.stringify(data)} not in enum ${JSON.stringify(schema.enum)}`);
    }
  }

  if (schema.type === "string") {
    if (schema.maxLength != null && data.length > schema.maxLength) {
      errors.push(`${path}: string longer than maxLength ${schema.maxLength}`);
    }
    if (schema.pattern != null && !new RegExp(schema.pattern).test(data)) {
      errors.push(`${path}: string does not match pattern /${schema.pattern}/`);
    }
  }

  if (schema.type === "array") {
    if (schema.minItems != null && data.length < schema.minItems) {
      errors.push(`${path}: array shorter than minItems ${schema.minItems}`);
    }
    if (schema.maxItems != null && data.length > schema.maxItems) {
      errors.push(`${path}: array longer than maxItems ${schema.maxItems}`);
    }
    if (schema.items) {
      data.forEach((el, i) => {
        const sub = validate(schema.items, el, `${path}[${i}]`);
        errors.push(...sub.errors);
      });
    }
  }

  // `required` / `properties` / `additionalProperties` apply to any object
  // value, even when the (sub-)schema has no explicit `type: "object"`. This
  // matters for oneOf branches that only specify `{required: [...]}`.
  if (data !== null && typeof data === "object" && !Array.isArray(data)) {
    if (schema.required) {
      for (const key of schema.required) {
        if (!(key in data)) {
          errors.push(`${path}: missing required property "${key}"`);
        }
      }
    }
    if (schema.additionalProperties === false && schema.properties) {
      for (const key of Object.keys(data)) {
        if (!(key in schema.properties)) {
          errors.push(`${path}: additional property "${key}" not allowed`);
        }
      }
    }
    if (schema.properties) {
      for (const [key, sub] of Object.entries(schema.properties)) {
        if (key in data) {
          const r = validate(sub, data[key], path ? `${path}.${key}` : key);
          errors.push(...r.errors);
        }
      }
    }
  }

  if (schema.oneOf) {
    const matches = schema.oneOf.filter((sub) => validate(sub, data, path).errors.length === 0);
    if (matches.length !== 1) {
      errors.push(`${path || "(root)"}: oneOf matched ${matches.length} branches (need exactly 1)`);
    }
  }

  return { valid: errors.length === 0, errors };
}
