package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	APIEndpoint = "https://api.anthropic.com/v1/messages"
	ModelHaiku  = "claude-haiku-4-5-20251001"
	ModelSonnet = "claude-sonnet-4-6"
	APIVersion  = "2023-06-01"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Response struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type ExtractedField struct {
	FieldName     string  `json:"field_name"`
	Value         string  `json:"value"`
	Confidence    float64 `json:"confidence"`
	Category      string  `json:"category"`
	OcrSourceText string  `json:"ocr_source_text,omitempty"`
}

type ExtractionResult struct {
	Fields     []ExtractedField
	TokensUsed int
}

// 핵심 원칙: OCR 텍스트만 Claude에 전송 (이미지 직접 전송 금지 — 토큰 85% 절감)
func ExtractDocumentFields(ocrText, documentType string) (*ExtractionResult, error) {
	systemPrompts := map[string]string{
		"BILL_OF_LADING": "무역 문서 파서. B/L에서 ERP 핵심 필드 추출. JSON만 반환. 설명 없음.",
		"COMMERCIAL_INVOICE": "무역 문서 파서. Invoice에서 ERP 핵심 필드 추출. JSON만 반환. 설명 없음.",
		"PACKING_LIST": "무역 문서 파서. Packing List에서 ERP 핵심 필드 추출. JSON만 반환. 설명 없음.",
		"INSURANCE": "무역 문서 파서. 보험증서에서 ERP 핵심 필드 추출. JSON만 반환. 설명 없음.",
		"CERTIFICATE": "무역 문서 파서. 증명서에서 ERP 핵심 필드 추출. JSON만 반환. 설명 없음.",
	}
	sys, ok := systemPrompts[documentType]
	if !ok { sys = "무역 문서 파서. 핵심 필드 추출. JSON만 반환." }

	userPrompt := fmt.Sprintf("다음 문서에서 ERP 핵심 필드를 추출해.\n반환 형식 (JSON만): {\"fields\":[{\"field_name\":\"한글필드명\",\"value\":\"값\",\"confidence\":0.0~1.0,\"category\":\"shipment_info|parties|cargo|financial|dates|reference\"}]}\n\n%s", ocrText)

	resp, err := Call(ModelHaiku, 800, sys, []Message{{Role: "user", Content: userPrompt}})
	if err != nil { return nil, err }

	text := CleanJSON(resp.Content[0].Text)
	var parsed struct { Fields []ExtractedField `json:"fields"` }
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("추출 결과 파싱 실패: %w", err)
	}
	return &ExtractionResult{Fields: parsed.Fields, TokensUsed: resp.Usage.InputTokens + resp.Usage.OutputTokens}, nil
}

func Chat(messages []Message, system string) (string, int, error) {
	resp, err := Call(ModelSonnet, 600, system, messages)
	if err != nil { return "", 0, err }
	return resp.Content[0].Text, resp.Usage.InputTokens + resp.Usage.OutputTokens, nil
}

func Call(model string, maxTokens int, system string, messages []Message) (*Response, error) {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	if apiKey == "" { return nil, fmt.Errorf("CLAUDE_API_KEY 없음") }
	body, _ := json.Marshal(map[string]interface{}{
		"model": model, "max_tokens": maxTokens, "system": system, "messages": messages,
	})
	req, _ := http.NewRequest("POST", APIEndpoint, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", APIVersion)
	resp, err := (&http.Client{}).Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 { return nil, fmt.Errorf("Claude API %d: %s", resp.StatusCode, string(respBytes)) }
	var result Response
	json.Unmarshal(respBytes, &result)
	return &result, nil
}

func CleanJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") { s = strings.TrimPrefix(s, "```json") }
	if strings.HasPrefix(s, "```") { s = strings.TrimPrefix(s, "```") }
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
