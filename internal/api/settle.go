package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
	"github.com/gin-gonic/gin"
	"snap-erp-api/internal/db"
	"snap-erp-api/internal/middleware"
)

func RegisterSettleRoutes(r *gin.RouterGroup) {
	r.GET("/ap", getAP)
	r.GET("/ar", getAR)
	r.GET("/exchange-rates", getExchangeRates)
}

func getAP(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	q := fmt.Sprintf("org_id=eq.%s&ap_status=in.(PENDING,PARTIAL,OVERDUE)&order=eta.asc&limit=%s",
		orgID, c.DefaultQuery("limit","20"))
	bls, _ := db.Default.Select("trade_bill_of_ladings", q)
	fxRate := getTodayRate(orgID)
	pendingKRW := 0.0
	for _, bl := range bls {
		if v, ok := bl["ap_balance"].(float64); ok { pendingKRW += v * fxRate }
	}
	c.JSON(200, gin.H{"data": bls, "pending_krw": pendingKRW, "fx_rate": fxRate, "label": "정산 예정금 (내가 받을 금액)"})
}

func getAR(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	q := fmt.Sprintf("org_id=eq.%s&ar_status=in.(PENDING,PARTIAL,OVERDUE)&order=ar_due_date.asc&limit=%s",
		orgID, c.DefaultQuery("limit","20"))
	rows, _ := db.Default.Select("trade_sales_assignments", q)
	c.JSON(200, gin.H{"data": rows, "label": "납부 예정금 (내가 낼 금액)"})
}

func getExchangeRates(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	today := time.Now().Format("2006-01-02")
	rate, _ := db.Default.SelectOne("trade_exchange_rates",
		fmt.Sprintf("org_id=eq.%s&date=eq.%s", orgID, today))
	if rate == nil {
		if fresh, err := fetchRate(); err == nil {
			fresh["org_id"] = orgID; fresh["date"] = today; fresh["auto_fetched"] = true
			db.Default.Insert("trade_exchange_rates", fresh)
			rate = fresh
		}
	}
	c.JSON(200, gin.H{"data": rate})
}

func fetchRate() (map[string]interface{}, error) {
	key := os.Getenv("EXCHANGE_RATE_API_KEY")
	if key == "" { return map[string]interface{}{"usd_krw": 1342.5}, nil }
	resp, err := http.Get("https://v6.exchangerate-api.com/v6/" + key + "/latest/USD")
	if err != nil { return nil, err }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct{ ConversionRates map[string]float64 `json:"conversion_rates"` }
	json.Unmarshal(body, &result)
	r := result.ConversionRates
	return map[string]interface{}{"usd_krw": r["KRW"], "eur_krw": r["KRW"]/r["EUR"], "jpy_krw": r["KRW"]/r["JPY"], "cny_krw": r["KRW"]/r["CNY"]}, nil
}

func getTodayRate(orgID string) float64 {
	today := time.Now().Format("2006-01-02")
	rate, _ := db.Default.SelectOne("trade_exchange_rates", fmt.Sprintf("org_id=eq.%s&date=eq.%s", orgID, today))
	if rate != nil { if v, ok := rate["usd_krw"].(float64); ok { return v } }
	return 1342.50
}
