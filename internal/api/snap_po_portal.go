package api

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"github.com/gin-gonic/gin"
	"snap-erp-api/internal/db"
	"snap-erp-api/internal/middleware"
)

func RegisterSnapRoutes(r *gin.RouterGroup) {
	snap := r.Group("/snap")
	snap.POST("/ingest", snapIngest)
	snap.GET("/status/:bl_id", snapStatus)
}

func snapIngest(c *gin.Context) {
	if c.GetHeader("X-Snap-Secret") != os.Getenv("SNAP_WEBHOOK_SECRET") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "인증 실패"}); return
	}
	var p struct {
		SnapReportID string `json:"snap_report_id"`
		ContainerNo  string `json:"container_no"`
		OrgID        string `json:"org_id"`
	}
	c.BindJSON(&p)
	norm := normalizeContainer(p.ContainerNo)
	bls, _ := db.Default.Select("trade_bill_of_ladings",
		fmt.Sprintf("org_id=eq.%s&status=neq.CLOSED", p.OrgID))
	now := time.Now().UTC().Format(time.RFC3339)
	for _, bl := range bls {
		if normalizeContainer(fmt.Sprintf("%v", bl["container_no"])) == norm {
			blID, _ := bl["id"].(string)
			db.Default.Update("trade_bill_of_ladings", "id=eq."+blID, map[string]interface{}{"updated_at": now})
			c.JSON(200, gin.H{"matched": true, "bl_id": blID, "container_no": norm}); return
		}
	}
	db.Default.Insert("trade_notifications", map[string]interface{}{
		"org_id": p.OrgID, "type": "SYSTEM", "severity": "important",
		"title": fmt.Sprintf("컨테이너 번호 확인 필요: %s", norm),
		"target_role": "staff", "is_read": false, "created_at": now,
	})
	c.JSON(200, gin.H{"matched": false, "container_no": norm})
}

func snapStatus(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	bl, _ := db.Default.SelectOne("trade_bill_of_ladings",
		fmt.Sprintf("id=eq.%s&org_id=eq.%s", c.Param("bl_id"), orgID))
	if bl == nil { c.JSON(404, gin.H{"error": "BL을 찾을 수 없어요"}); return }
	c.JSON(200, gin.H{"bl_id": c.Param("bl_id"), "bl": bl})
}

func normalizeContainer(raw string) string {
	return strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9]`).ReplaceAllString(raw, ""))
}

func RegisterPORoutes(r *gin.RouterGroup) {
	pos := r.Group("/pos")
	pos.GET("", func(c *gin.Context) {
		orgID := middleware.GetOrgID(c)
		pos, _ := db.Default.Select("trade_purchase_orders", fmt.Sprintf("org_id=eq.%s&order=created_at.desc&limit=20", orgID))
		c.JSON(200, gin.H{"data": pos})
	})
	pos.POST("", func(c *gin.Context) {
		orgID := middleware.GetOrgID(c)
		var body map[string]interface{}; c.BindJSON(&body)
		body["org_id"] = orgID; body["manager_user_id"] = middleware.GetUserID(c)
		po, err := db.Default.Insert("trade_purchase_orders", body)
		if err != nil { c.JSON(500, gin.H{"error": "잠깐, 다시 시도해 주세요"}); return }
		c.JSON(201, gin.H{"data": po})
	})
}

func RegisterPortalRoutes(r *gin.RouterGroup) {
	portal := r.Group("/portal")
	supplier := portal.Group("/supplier")
	supplier.Use(middleware.RequireRole("supplier","admin","manager"))
	supplier.GET("/home", func(c *gin.Context) {
		orgID := middleware.GetOrgID(c)
		bls, _ := db.Default.Select("trade_bill_of_ladings",
			fmt.Sprintf("org_id=eq.%s&status=in.(PLANNED,SHIPPED,ARRIVED)&select=id,bl_number,eta,status,ap_status,ap_amount,ap_balance&limit=5", orgID))
		c.JSON(200, gin.H{"active_bls": bls, "label_ap": "정산 예정금 (내가 받을 금액)"})
	})

	buyer := portal.Group("/buyer")
	buyer.Use(middleware.RequireRole("buyer","admin","manager"))
	buyer.GET("/home", func(c *gin.Context) {
		orgID := middleware.GetOrgID(c)
		contracts, _ := db.Default.Select("trade_sales_contracts",
			fmt.Sprintf("org_id=eq.%s&status=in.(ACTIVE,PARTIAL)&limit=5", orgID))
		c.JSON(200, gin.H{"active_contracts": contracts, "label_ar": "납부 예정금 (내가 낼 금액)"})
	})

	portal.POST("/role/switch", func(c *gin.Context) {
		var req struct{ TargetRole string `json:"target_role"` }; c.BindJSON(&req)
		db.Default.Update("trade_users", "id=eq."+middleware.GetUserID(c), map[string]interface{}{"last_role_used": req.TargetRole})
		c.JSON(200, gin.H{"active_role": req.TargetRole, "message": "역할이 변경됐습니다"})
	})
}
