package grifts

// Grift tasks are self-registered via grift.Add / grift.Namespace in the
// individual task files (seed.go, consolidation.go, ...).
//
// Note: buffalo v1 (v1.1.4) removed the built-in `routes`, `middleware` and
// `secret` helper grifts that used to be registered by `buffalo.Grifts(app)`;
// there is no equivalent registration needed anymore.
