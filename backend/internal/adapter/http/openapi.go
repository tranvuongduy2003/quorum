// @title                   Quorum API
// @version                 1.0.0
// @description             The service foundation for Quorum: health probes, diagnostics routes, one shared error envelope and one shared rate-limit budget. Every failure except the readiness report uses the same envelope, and every response carries an X-Request-Id you can quote in a bug report.
// @servers.url             http://localhost:8080
// @servers.description     Local development
// @tag.name                health
// @tag.description         Liveness and readiness probes. Unversioned, never rate limited, and meant for orchestrators rather than for people.
// @tag.name                diagnostics
// @tag.description         Versioned routes that exercise the service end to end: the router, the error envelope, the rate limiter and the PostgreSQL pool.
// @tag.name                debug
// @tag.description         Routes that exist to provoke failure. Registered only when DEBUG_ROUTES_ENABLED is true, so treat them as absent unless you know otherwise.
// @tag.name                documentation
// @tag.description         This document and the reference page rendered from it. Registered only when DOCS_ENABLED is true.
// @externalDocs.url        https://spec.openapis.org/oas/v3.1.0.html
// @externalDocs.description OpenAPI 3.1.0, the specification this document follows
package deliveryhttp
