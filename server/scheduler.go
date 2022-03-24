// Copyright 2020 The Nakama Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ticker = make(map[string]*time.Ticker)
var done = make(map[string]chan string)

// Schedules the execution of the supplied RPC.
func SchedulerInSeconds(logger *zap.Logger, rpcName string, delaySeconds int64, data string, key string, async bool) (bool, error) {
	logger.Info("core_scheduler.go: SchedulerInSeconds: STARTING " + key)

	if maybeJSON := []byte(data); !json.Valid(maybeJSON) || bytes.TrimSpace(maybeJSON)[0] != byteBracket {
		return false, status.Error(codes.InvalidArgument, "Value must be a JSON object.")
	}

	_, exist := ticker[key]
	if exist {
		SchedulerCancel(key)
	}

	done[key] = make(chan string)

	ticker[key] = schedule(httpPost, logger, rpcName, time.Duration(delaySeconds), data, key, async, done[key])

	return true, nil
}

// Cancels the execution of a previously scheduled RPC by key
func SchedulerCancel(key string) {
	_, exist := ticker[key]

	if exist && !(isClosed(done[key])) {
		close(done[key])
		ticker[key].Stop()
	}
}

func schedule(f func(*zap.Logger, string, string), logger *zap.Logger, rpcName string, interval time.Duration, data string, key string, async bool, done <-chan string) *time.Ticker {
	ticker := time.NewTicker(interval * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				if async {
					go f(logger, rpcName, data)
				} else {
					f(logger, rpcName, data)
				}
			case <-done:
				logger.Info("core_scheduler.go: SchedulerInSeconds: QUIT " + key)
				return
			}
		}
	}()
	return ticker
}

func httpPost(logger *zap.Logger, rpcName string, data string) {
	json_data, err := json.Marshal(data)

	if err != nil {
		logger.Error("core_scheduler.go: SchedulerInSeconds: httpPost", zap.Error(err))
	}

	resp, err := http.Post("http://127.0.0.1:7350/v2/rpc/"+rpcName+"?http_key=defaulthttpkey", "application/json",
		bytes.NewBuffer(json_data))

	if err != nil {
		logger.Error("core_scheduler.go: SchedulerInSeconds: httpPost", zap.Error(err))
	}

	defer resp.Body.Close()

	// if resp.StatusCode == http.StatusOK {
	// 	bodyBytes, err := io.ReadAll(resp.Body)
	// 	if err != nil {
	// 		logger.Error("core_scheduler.go: SchedulerInSeconds: httpPost", zap.Error(err))
	// 	}
	// 	bodyString := string(bodyBytes)
	// 	logger.Info("core_scheduler.go: SchedulerInSeconds: httpPost: RESPONSE: " + bodyString)
	// }
}

func isClosed(ch <-chan string) bool {
	select {
	case <-ch:
		return true
	default:
	}

	return false
}
