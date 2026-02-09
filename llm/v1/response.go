package v1

import (
	"bufio"
	"encoding/json"

	"github.com/bincooo/ago/logger"
	"github.com/bincooo/ago/model"

	"github.com/go-resty/resty/v2"
)

type chunkBodies struct {
	chunk string
	think string
}

func waitChannel(ctx *model.Ctx, response *resty.Response) *chunkBodies {
	channel := createChannel(ctx, response)
	var newBodies chunkBodies
	for {
		bodies, ok := <-channel
		if !ok {
			break
		}
		newBodies.chunk += bodies.chunk
		newBodies.think = bodies.think
	}
	return &newBodies
}

func createChannel(ctx *model.Ctx, response *resty.Response) chan *chunkBodies {
	channel := make(chan *chunkBodies)
	go func() {
		defer close(channel)

		raw := response.RawResponse
		scanner := bufio.NewScanner(raw.Body)
		for {
			if ctx.IsConnClosed() {
				logger.Sugar().Errorf("客户端已断开连接，停止处理")
				return
			}

			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					logger.Sugar().Error(err)
				}
				break
			}

			data := scanner.Text()
			//logger.Sugar().Debugf("--------- ORIGINAL MESSAGE ---------")
			//logger.Sugar().Debugf("%s", data)

			if len(data) < 6 || data[:6] != "data: " {
				continue
			}

			data = data[6:]
			if data == "[DONE]" {
				break
			}

			var chat model.Response
			err := json.Unmarshal([]byte(data), &chat)
			if err != nil {
				logger.Sugar().Error(err)
				continue
			}

			if len(chat.Choices) == 0 {
				continue
			}

			choice := chat.Choices[0]
			if choice.Delta.Role != "" && choice.Delta.Role != "assistant" {
				continue
			}

			if choice.FinishReason != nil && *choice.FinishReason == "stop" {
				continue
			}

			chunk := choice.Delta.Content
			logger.Sugar().Debug("----- raw -----")
			logger.Sugar().Debug(chunk)

			if len(chunk) == 0 {
				continue
			}

			channel <- &chunkBodies{
				chunk: chunk,
			}
		}
	}()

	return channel
}
