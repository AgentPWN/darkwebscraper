package utils

type CompanyGunra struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type CompanyIncRansom struct {
	ID      string `json:"_id"`
	Company struct {
		Name string `json:"company_name"`
	} `json:"company"`
	Desc []string `json:"description"`
}

type CompanyKairos struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Desc string `json:"info"`
}

type CompanyLamashtu struct {
	ID   string `json:"id"`
	Name string `json:"title"`
	Desc string `json:"short_desc"`
}

type CompanyLinkcpub struct {
	ID   string `json:"articleId"`
	Name string `json:"title"`
	Desc string `json:"description"`
}

type CompanyKillSec struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CompanyLynx struct {
	ID      string `json:"_id"`
	Company struct {
		Name string `json:"company_name"`
	} `json:"company"`
	Desc []string `json:"description"`
}

type CompanyMoneyMessage struct {
	ID   int    `json:"pageId"`
	Name string `json:"name"`
}

type CompanyMorpheus struct {
	ID   int    `json:"id"`
	Name string `json:"title"`
}

type CompanySinobi struct {
	ID      string `json:"_id"`
	Company struct {
		Name string `json:"company_name"`
	} `json:"company"`
	Desc []string `json:"description"`
}

type CompanyTermite struct {
	ID   string `json:"_id"`
	Name string `json:"title"`
	Desc string `json:"description"`
}

type CompanyWarlock struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Desc string `json:"description"`
}

type CompanyAbyss struct {
	Title string   `json:"title"`
	Short string   `json:"short"`
	Full  string   `json:"full"`
	Links []string `json:"links"`
}

type CompanyAkira struct {
	Title string `json:"title"`
	Name  string `json:"name"`
	Desc  string `json:"desc"`
	Links string `json:"links"`
}
