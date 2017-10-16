package routes

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

func unmarshalRequest(ctx *fasthttp.RequestCtx, v interface{}) error {
	return json.Unmarshal(ctx.Request.Body(), &v)
}
