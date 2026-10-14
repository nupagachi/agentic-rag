// Package httpx holds the HTTP building blocks shared by every API domain:
// the base middlewares (request id, access log, recover, CORS) and helpers to
// read request-scoped values from the context.
//
// Middleware order, outermost first, is RequestID, AccessLog, Recover, CORS.
// server.Middlewares assembles them in that order.
package httpx
