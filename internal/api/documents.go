package api

import (
	"fmt"
	"io"
	"net/http"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"snap-erp-api/internal/db"
	"snap-erp-api/internal/middleware"
	"snap-erp-api/internal/pipeline"
)

func RegisterDocumentRoutes(r *gin.RouterGroup) {
	d := r.Group("/documents")
	d.POST("/upload", uploadDocument)
	d.GET("/queue", getDocumentQueue)
	d.GET("/:id/extraction", getExtraction)
	d.POST("/:id/confirm", confirmDocument)
	d.POST("/:id/reject", rejectDocument)
}

func uploadDocument(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	userID := middleware.GetUserID(c)
	file, header, err := c.Request.FormFile("file")
	if err != nil { c.JSON(400, gin.H{"error": "파일을 올려주세요"}); return }
	defer file.Close()
	if header.Size > 50*1024*1024 { c.JSON(400, gin.H{"error": "파일이 너무 커요 (50MB 이하)"}); return }
	content, _ := io.ReadAll(file)
	docType := pipeline.DetectDocType(header.Filename)
	if p := c.PostForm("document_type"); p != "" { docType = pipeline.DocumentType(p) }
	documentID := uuid.New().String()
	db.Default.Insert("trade_documents", map[string]interface{}{
		"id": documentID, "org_id": orgID, "uploader_id": userID,
		"doc_type": "OTHER", "intake_source": "upload",
		"status": "parsing", "processing_stage": 0,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})
	pipeline.RunAsync(documentID, orgID, content, header.Filename, docType)
	c.JSON(200, gin.H{"document_id": documentID, "status": "processing", "document_type": string(docType)})
}

func getDocumentQueue(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	docs, err := db.Default.Select("trade_documents",
		fmt.Sprintf("org_id=eq.%s&processing_stage=eq.5&order=created_at.desc&limit=20", orgID))
	if err != nil { c.JSON(500, gin.H{"error": "잠깐, 다시 시도해 주세요"}); return }
	c.JSON(200, gin.H{"data": docs})
}

func getExtraction(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	docID := c.Param("id")
	doc, err := db.Default.SelectOne("trade_documents", fmt.Sprintf("id=eq.%s&org_id=eq.%s", docID, orgID))
	if err != nil || doc == nil { c.JSON(404, gin.H{"error": "문서를 찾을 수 없어요"}); return }
	fields, _ := db.Default.Select("trade_extraction_fields", fmt.Sprintf("document_id=eq.%s&order=created_at.asc", docID))
	c.JSON(200, gin.H{"document": doc, "fields": fields})
}

// Confirm Card — AI 제안을 사람이 확인 후 저장 (P2 원칙: 이 경로 없이 DB 쓰기 없음)
func confirmDocument(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	userID := middleware.GetUserID(c)
	docID := c.Param("id")
	var req struct {
		FieldIDs []string          `json:"field_ids"`
		Edits    map[string]string `json:"edits"`
	}
	c.BindJSON(&req)
	doc, err := db.Default.SelectOne("trade_documents", fmt.Sprintf("id=eq.%s&org_id=eq.%s", docID, orgID))
	if err != nil || doc == nil { c.JSON(404, gin.H{"error": "문서를 찾을 수 없어요"}); return }
	fields, _ := db.Default.Select("trade_extraction_fields", fmt.Sprintf("document_id=eq.%s", docID))
	now := time.Now().UTC().Format(time.RFC3339)
	confirmedCount := 0
	writtenFields := []map[string]interface{}{}
	for _, field := range fields {
		fieldID, _ := field["id"].(string)
		if len(req.FieldIDs) > 0 && !contains(req.FieldIDs, fieldID) { continue }
		finalValue, _ := field["extracted_value"].(string)
		userEdited := false
		if v, ok := req.Edits[fieldID]; ok { finalValue = v; userEdited = true }
		db.Default.Update("trade_extraction_fields", "id=eq."+fieldID, map[string]interface{}{
			"confirmed": true, "confirmed_by": userID, "confirmed_at": now,
			"final_value": finalValue, "user_edited": userEdited,
		})
		db.Default.Insert("trade_audit_logs", map[string]interface{}{
			"org_id": orgID, "user_id": userID, "action": "FIELD_CONFIRMED",
			"entity_type": "trade_extraction_fields", "entity_id": fieldID,
			"source": "ai_action", "created_at": now,
		})
		confirmedCount++
		writtenFields = append(writtenFields, map[string]interface{}{"field": field["field_name"], "value": finalValue})
	}
	db.Default.Update("trade_documents", "id=eq."+docID, map[string]interface{}{"status": "approved", "processing_stage": 6})
	c.JSON(200, gin.H{"confirmed_count": confirmedCount, "written_fields": writtenFields, "message": "저장됐습니다"})
}

func rejectDocument(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	docID := c.Param("id")
	var req struct{ Reason string `json:"reason"` }
	c.BindJSON(&req)
	now := time.Now().UTC().Format(time.RFC3339)
	db.Default.Update("trade_extraction_fields", "document_id=eq."+docID, map[string]interface{}{"confirmed": false, "confirmed_at": now})
	db.Default.Update("trade_documents", "id=eq."+docID, map[string]interface{}{"status": "rejected", "rejected_reason": req.Reason})
	db.Default.Insert("trade_audit_logs", map[string]interface{}{
		"org_id": orgID, "action": "DOCUMENT_REJECTED",
		"entity_type": "trade_documents", "entity_id": docID,
		"source": "ai_action", "created_at": now,
	})
	c.JSON(200, gin.H{"status": "rejected", "message": "건너뛰었습니다"})
}

func contains(s []string, v string) bool {
	for _, x := range s { if x == v { return true } }
	return false
}
