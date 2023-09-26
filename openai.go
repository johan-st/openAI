package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Credentials struct {
	ApiKey string
}

var (
	// API endpoints
	urlChatCompletion = "https://api.openai.com/v1/chat/completions"
)

func translate(key, lang, text string) (string, error) {

	openai := "https://api.openai.com/v1/chat/completions"

	type reqRespMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type reqType struct {
		Model    string           `json:"model"`
		Messages []reqRespMessage `json:"messages"`
	}

	type respChoices struct {
		Index        int            `json:"index"`
		Message      reqRespMessage `json:"message"`
		FinishReason string         `json:"finish_reason"`
	}

	type respUsage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}

	type respType struct {
		ID      string        `json:"id"`
		Object  string        `json:"object"`
		Created int           `json:"created"`
		Model   string        `json:"model"`
		Choices []respChoices `json:"choices"`
		Usage   respUsage     `json:"usage"`
	}

	prompt := fmt.Sprintf(`Translate the following text into %s: %s`, lang, text)

	reqStruct := reqType{
		Model: "gpt-3.5-turbo",
		Messages: []reqRespMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	reqBytes, err := json.Marshal(reqStruct)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", openai, bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respStruct := respType{}
	err = json.NewDecoder(resp.Body).Decode(&respStruct)
	if err != nil {
		return "", err
	}

	return respStruct.Choices[0].Message.Content, nil
}
