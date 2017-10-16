package routes

import (
	"fmt"

	"github.com/valyala/fasthttp"
)

// Index route (GET /v1). Used for simple status checks.
func Index(ctx *fasthttp.RequestCtx) {
	fmt.Fprint(ctx, `"OK"`)
}
