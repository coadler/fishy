package routes

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

func respond(ctx *fasthttp.RequestCtx, data interface{}) {
	e := json.NewEncoder(ctx)
	e.SetEscapeHTML(false)
	e.Encode(APIResponse{false, "", data})
}

func respondMessage(ctx *fasthttp.RequestCtx, isErr bool, err string) {
	json.NewEncoder(ctx).Encode(APIResponse{isErr, err, ""})
}
