package model

type Region struct {
	ID       int32  `json:"id"`
	NameAr   string `json:"name_ar"`
	NameEn   string `json:"name_en"`
	IsActive bool   `json:"is_active"`
}

type Interest struct {
	ID       int32  `json:"id"`
	NameAr   string `json:"name_ar"`
	NameEn   string `json:"name_en"`
	IsActive bool   `json:"is_active"`
}

type Job struct {
	ID       int32  `json:"id"`
	NameAr   string `json:"name_ar"`
	NameEn   string `json:"name_en"`
	Key      string `json:"key"`
	IsActive bool   `json:"is_active"`
}
