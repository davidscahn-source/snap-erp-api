# ECOYA SNAP ERP API

Go + Supabase + Claude AI 기반 무역 문서 자동화 ERP API

## 빠른 시작

```bash
cp .env.example .env
go mod tidy
make run
```

## 핵심 설계 원칙
1. 이미지 → Claude 직접 전송 ❌, OCR 텍스트 → Claude ✅ (토큰 85% 절감)
2. AI 제안, 사람 결정: Confirm Card 없이 DB 쓰기 ❌
3. org_id 격리: 서버 주입, 사용자 재정의 ❌
4. 파트너 필드 억제: 원가/마진 파트너에게 ❌

## API Endpoints
| Method | Path | 설명 |
|--------|------|------|
| POST | /api/v1/documents/upload | 문서 업로드 → 파이프라인 |
| GET | /api/v1/documents/queue | HITL 대기 목록 |
| POST | /api/v1/documents/:id/confirm | Confirm → DB 저장 |
| GET | /api/v1/bls | BL 목록 |
| GET | /api/v1/ap | 정산 예정금 |
| GET | /api/v1/ar | 납부 예정금 |
| POST | /api/v1/snap/ingest | SNAP → BL 매칭 |
