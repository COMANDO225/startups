package ulid

import "github.com/gofiber/fiber/v3"

// RouteConstraint validates ULID path parameters at the router level.
// Invalid ULIDs get a 404 before reaching any handler or database.
//
// Usage in routes:
//
//	app.Get("/staff/:id<ulid>", handler)
type RouteConstraint struct{}

func (RouteConstraint) Name() string { return "ulid" }

func (RouteConstraint) Execute(param string, _ ...string) bool {
	_, err := Parse(param)
	return err == nil
}

var _ fiber.CustomConstraint = RouteConstraint{}
