package pipeline

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"path/filepath"

	"snap-erp-api/internal/claude"
	"snap-erp-api/internal/db"
)

type DocumentType string

const (
	DocTypeBL          DocumentType = "BILL_OF_LADING"
	DocTypePackingList DocumentType = "PACKING_LIST"
	DocTypeInvoice     DocumentType = "COMMERCIAL_INVOICE"
	DocTypeInsurance   DocumentType = "INSURANCE"
	DocTypeCert        DocumentType = "CERTIFICATE"
	DocTypeUnknown     DocumentType = "UNKNOWN"
)

const (
	StageIntake     = 0
	StagePreprocess = 1
	StageOCR        = 2
	StageExtraction = 3
	StageValidation = 4
	StageHITL       = 5
	StageCompleted  = 6
)

func DetectDocType(filename string) DocumentType {
	n := strings.ToLower(filename)
	switch {
	case strings.Contains(n, "bl") || strings.Contains(n, "bill"):
		return DocTypeBL
	case strings.Contains(n, "pack") || strings.Contains(n, "pl_"):
		return DocTypePackingList
	case strings.Contains(n, "inv"):
		return DocTypeInvoice
	case strings.Contains(n, "insur"):
		return DocTypeInsurance
	case strings.Contains(n, "cert") || strings.Contains(n, "wqc"):
		return DocTypeCert
	default:
		return DocTypeUnknown
	}
}

func RunAsync(documentID, orgID string, fileContent []byte, filename string, docType DocumentType) {
	go func() {
		if err := run(documentID, orgID, fileContent, filename, docType); err != nil {
			log.Printf("❌ 파이프라인 오류 [%s]: %v", documentID, err)
			db.Default.Update("trade_documents", "id=eq."+documentID, map[string]interface{}{
				"status": "failed", "rejected_reason": err.Error(),
			})
		}
	}()
}

func run(documentID, orgID string, fileContent []byte, filename string, docType DocumentType) error {
	update := func(stage int) {
		db.Default.Update("trade_documents", "id=eq."+documentID, map[string]interface{}{"processing_stage": stage})
	}

	update(StagePreprocess)
	update(StageOCR)

	// OCR: 텍스트 파일이면 직접 사용, 아니면 GCV
	var ocrText string
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".txt" || ext == ".csv" {
		ocrText = string(fileContent)
	} else {
		// TODO: Google Cloud Vision API 연동
		ocrText = string(fileContent) // 개발환경 폴백
	}

	update(StageExtraction)
	log.Printf("🤖 [%s] Claude Haiku 추출 시작 (토큰 절감 모드)", documentID)

	extraction, err := claude.ExtractDocumentFields(ocrText, string(docType))
	if err != nil { return fmt.Errorf("추출 실패: %w", err) }

	log.Printf("✅ [%s] 추출 완료: %d개 필드, %d tokens", documentID, len(extraction.Fields), extraction.TokensUsed)

	fieldsJSON, _ := json.Marshal(extraction.Fields)
	avgConf := calcAvgConf(extraction.Fields)

	db.Default.Update("trade_documents", "id=eq."+documentID, map[string]interface{}{
		"extraction_result":  string(fieldsJSON),
		"confidence_overall": avgConf,
	})

	for _, field := range extraction.Fields {
		db.Default.Insert("trade_extraction_fields", map[string]interface{}{
			"document_id":     documentID,
			"field_name":      field.FieldName,
			"target_table":    getTargetTable(docType),
			"extracted_value": field.Value,
			"confidence":      field.Confidence,
			"ocr_source_text": field.OcrSourceText,
			"confirmed":       nil,
		})
	}

	db.Default.Insert("trade_ai_sessions", map[string]interface{}{
		"org_id": orgID, "session_type": "file_ocr",
		"token_used": extraction.TokensUsed, "confirmed": false,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})

	update(StageValidation)

	needsHITL := avgConf < 0.95 || hasMoneyField(extraction.Fields)
	if needsHITL {
		update(StageHITL)
		db.Default.Insert("trade_notifications", map[string]interface{}{
			"org_id": orgID, "type": "DOC_REVIEW", "severity": "important",
			"title": fmt.Sprintf("문서 확인 필요 — %s", filename),
			"target_role": "staff", "is_read": false,
		})
	} else {
		update(StageCompleted)
		db.Default.Update("trade_documents", "id=eq."+documentID, map[string]interface{}{"status": "approved"})
	}
	return nil
}

func calcAvgConf(fields []claude.ExtractedField) float64 {
	if len(fields) == 0 { return 0 }
	sum := 0.0
	for _, f := range fields { sum += f.Confidence }
	return sum / float64(len(fields))
}

func hasMoneyField(fields []claude.ExtractedField) bool {
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f.FieldName), "금액") ||
			strings.Contains(strings.ToLower(f.FieldName), "amount") ||
			strings.Contains(strings.ToLower(f.FieldName), "price") { return true }
	}
	return false
}

func getTargetTable(d DocumentType) string {
	switch d {
	case DocTypeBL, DocTypePackingList, DocTypeCert: return "trade_bill_of_ladings"
	case DocTypeInvoice: return "trade_sales_assignments"
	default: return "trade_documents"
	}
}
