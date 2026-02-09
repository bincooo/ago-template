package v1

import (
	"fmt"
	"io"
	"time"

	"github.com/bincooo/ago"
	"github.com/bincooo/ago/model"
)

var (
	schema = make([]model.Record[string, any], 0)
	Sdk    = ago.Sdk()
)

func init() {
	Sdk.OnInitialized(func() {
		environ := Sdk.Env()
		llm := environ.Get("custom-llm")
		var modelSlice []string
		if slice, ok := llm.([]interface{}); ok {
			for _, it := range slice {
				var rec model.Record[string, any]
				rec, ok = it.(map[string]interface{})
				if !ok {
					continue
				}

				// validate
				_, ok = model.GetValue[string, string](rec, "reversal")
				if !ok {
					panic("`reversal` not found in config.yaml ==> custom-llm")
				}
				schema = append(schema, rec)

				prefix := model.JustValue[string, string](rec, "prefix")
				models := model.JustValue[string, []interface{}](rec, "model")
				for _, mod := range models {
					modelSlice = append(modelSlice, fmt.Sprintf("%s/%s", prefix, mod))
				}
			}
		}

		Sdk.Support(modelSlice...).
			Relay(func(ctx *model.Ctx) (err error) {
				completion := model.JustValue[string, *model.Completion](ctx.Record, "completion")
				unix := time.Now().Unix()

				response, err := fetch(ctx)
				if err != nil {
					return err
				}

				if completion.Stream {
					ctx.StreamWriter(func(w func(interface{}) error) {
						chunkChan := createChannel(ctx, response)
						for {
							bodies, ok := <-chunkChan
							if !ok {
								err = w(io.EOF)
								return
							}

							err = w(model.CreateStreamResponse(bodies.chunk, unix))
						}
					})
					return
				}

				bodies := waitChannel(ctx, response)
				return ctx.Writer(model.CreateResponse(bodies.chunk, unix))
			})
	})
}
