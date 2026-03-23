package api

import (
	"fmt"
	"net/http"
	"github.com/gin-gonic/gin"
	"snap-erp-api/internal/db"
	"snap-erp-api/internal/middleware"
)

func RegisterBLRoutes(r *gin.RouterGroup) {
	bls := r.Group("/bls")
	bls.GET("", listBLs)
	bls.GET("/:id", getBL)
	bls.POST("", createBL)
	bls.PATCH("/:id", updateBL)
}

func listBLs(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	q := fmt.Sprintf("org_id=eq.%s&order=created_at.desc&limit=%s&offset=%s",
		orgID, c.DefaultQuery("limit","20"), c.DefaultQuery("offset","0"))
	if s := c.Query("status"); s != "" { q += "&status=eq." + s }
	bls, err := db.Default.Select("trade_bill_of_ladings", q)
	if err != nil { c.JSON(500, gin.H{"error": "잠깐, 다시 시도해 주세요"}); return }
	c.JSON(200, gin.H{"data": bls, "count": len(bls)})
}

func getBL(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	bl, err := db.Default.SelectOne("trade_bill_of_ladings",
		fmt.Sprintf("id=eq.%s&org_id=eq.%s", c.Param("id"), orgID))
	if err != nil || bl == nil { c.JSON(404, gin.H{"error": "BL을 찾을 수 없어요"}); return }
	// 파트너에게 원가 제거
	pr, _ := c.Get("primary_role")
	if r, _ := pr.(string); r == "supplier" || r == "buyer" {
		delete(bl, "purchase_cost_krw"); delete(bl, "freight"); delete(bl, "customs_fee")
	}
	c.JSON(200, gin.H{"data": bl})
}

func createBL(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	var body map[string]interface{}
	c.BindJSON(&body)
	body["org_id"] = orgID
	body["manager_user_id"] = middleware.GetUserID(c)
	bl, err := db.Default.Insert("trade_bill_of_ladings", body)
	if err != nil { c.JSON(500, gin.H{"error": "잠깐, 다시 시도해 주세요"}); return }
	c.JSON(201, gin.H{"data": bl})
}

func updateBL(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	bl, err := db.Default.SelectOne("trade_bill_of_ladings",
		fmt.Sprintf("id=eq.%s&org_id=eq.%s", c.Param("id"), orgID))
	if err != nil || bl == nil { c.JSON(404, gin.H{"error": "BL을 찾을 수 없어요"}); return }
	var body map[string]interface{}
	c.BindJSON(&body)
	delete(body, "org_id"); delete(body, "id")
	db.Default.Update("trade_bill_of_ladings", fmt.Sprintf("id=eq.%s&org_id=eq.%s", c.Param("id"), orgID), body)
	c.JSON(200, gin.H{"message": "저장됐습니다"})
}
