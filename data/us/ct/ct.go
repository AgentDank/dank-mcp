// Copyright 2025 Neomantra Corp
//
// CT Cannabis Data
//
// Socrata Documentation:
//   https://dev.socrata.com/foundry/data.ct.gov/egd5-wb6r
// Interactive Brand Dataset:
//   https://data.ct.gov/api/views/egd5-wb6r/rows.csv?accessType=DOWNLOAD&api_foundry=true

package ct

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AgentDank/dank-mcp/data"
	"github.com/AgentDank/dank-mcp/internal/db"
	"github.com/relvacode/iso8601"
)

const (
	BRAND_JSON_FILENAME = "us_ct_brands.json"
	BRAND_CSV_FILENAME  = "us_ct_brands.csv"
	// CTBrandsURL is the URL to fetch the CT cannabis brands data
	BrandsURL = "https://data.ct.gov/resource/egd5-wb6r.json"
)

type Image struct {
	URL         string `csv:"url" json:"url"`          // URL is the URL to the image
	Description string `csv:"desc" json:"description"` // Description is the description of the image
}

// Raw Cannabis Brand Record
type Brand struct {
	BrandName                    string       `csv:"BRAND-NAME" json:"brand_name"`
	DosageForm                   string       `csv:"DOSAGE-FORM" json:"dosage_form"`
	BrandingEntity               string       `csv:"BRANDING-ENTITY" json:"branding_entity"`
	ProductImage                 Image        `csv:"PRODUCT-IMAGE" json:"product_image"`
	LabelImage                   Image        `csv:"LABEL-IMAGE" json:"label_image"`
	LabAnalysis                  Image        `csv:"LAB-ANALYSIS" json:"lab_analysis"`
	ApprovalDate                 iso8601.Time `csv:"APPROVAL-DATE" json:"approval_date"`
	RegistrationNumber           string       `csv:"REGISTRATION-NUMBER" json:"registration_number"`
	TetrahydrocannabinolThc      Measure      `csv:"TETRAHYDROCANNABINOL-THC" json:"tetrahydrocannabinol_thc"`
	TetrahydrocannabinolAcidThca Measure      `csv:"TETRAHYDROCANNABINOL-ACID-THCA" json:"tetrahydrocannabinol_acid_thca"`
	CannabidiolsCbd              Measure      `csv:"CANNABIDIOLS-CBD" json:"cannabidiols_cbd"`
	CannabidiolAcidCbda          Measure      `csv:"CANNABIDIOL-ACID-CBDA" json:"cannabidiol_acid_cbda"`
	APinene                      Measure      `csv:"A-PINENE" json:"a_pinene"`
	BMyrcene                     Measure      `csv:"B-MYRCENE" json:"b_myrcene"`
	BCaryophyllene               Measure      `csv:"B-CARYOPHYLLENE" json:"b_caryophyllene"`
	BPinene                      Measure      `csv:"B-PINENE" json:"b_pinene"`
	Limonene                     Measure      `csv:"LIMONENE" json:"limonene"`
	Ocimene                      Measure      `csv:"OCIMENE" json:"ocimene"`
	LinaloolLin                  Measure      `csv:"LINALOOL-LIN" json:"linalool_lin"`
	HumuleneHum                  Measure      `csv:"HUMULENE-HUM" json:"humulene_hum"`
	Cbg                          Measure      `csv:"CBG" json:"cbg"`
	CbgA                         Measure      `csv:"CBG-A" json:"cbg_a"`
	CannabavarinCbdv             Measure      `csv:"CANNABAVARIN-CBDV" json:"cannabavarin_cbdv"`
	CannabichromeneCbc           Measure      `csv:"CANNABICHROMENE-CBC" json:"cannabichromene_cbc"`
	CannbinolCbn                 Measure      `csv:"CANNBINO-CBN" json:"cannbinol_cbn"`
	TetrahydrocannabivarinThcv   Measure      `csv:"TETRAHYDROCANNABIVARIN-THCV" json:"tetrahydrocannabivarin_thcv"`
	ABisabolol                   Measure      `csv:"A-BISABOLOL" json:"a_bisabolol"`
	APhellandrene                Measure      `csv:"A-PHELLANDRENE" json:"a_phellandrene"`
	ATerpinene                   Measure      `csv:"A-TERPINENE" json:"a_terpinene"`
	BEudesmol                    Measure      `csv:"B-EUDESMOL" json:"b_eudesmol"`
	BTerpinene                   Measure      `csv:"B-TERPINENE" json:"b_terpinene"`
	Fenchone                     Measure      `csv:"FENCHONE" json:"fenchone"`
	Pulegol                      Measure      `csv:"PULEGOL" json:"pulegol"`
	Borneol                      Measure      `csv:"BORNEOL" json:"borneol"`
	Isopulegol                   Measure      `csv:"ISOPULEGOL" json:"isopulegol"`
	Carene                       Measure      `csv:"CARENE" json:"carene"`
	Camphene                     Measure      `csv:"CAMPHENE" json:"camphene"`
	Camphor                      Measure      `csv:"CAMPHOR" json:"camphor"`
	CaryophylleneOxide           Measure      `csv:"CARYOPHYLLENE_OXIDE" json:"caryophyllene_oxide"`
	Cedrol                       Measure      `csv:"CEDROL" json:"cedrol"`
	Eucalyptol                   Measure      `csv:"EUCALYPTOL" json:"eucalyptol"`
	Geraniol                     Measure      `csv:"GERANIOL" json:"geraniol"`
	Guaiol                       Measure      `csv:"GUAIOL" json:"guaiol"`
	GeranylAcetate               Measure      `csv:"GERANYL_ACETATE" json:"geranyl_acetate"`
	Isoborneol                   Measure      `csv:"ISOBORNEOL" json:"isoborneol"`
	Menthol                      Measure      `csv:"MENTHOL" json:"menthol"`
	LFenchone                    Measure      `csv:"L-FENCHONE" json:"l_fenchone"`
	Nerol                        Measure      `csv:"NEROL" json:"nerol"`
	Sabinene                     Measure      `csv:"SABINENE" json:"sabinene"`
	Terpineol                    Measure      `csv:"TERPINEOL" json:"terpineol"`
	Terpinolene                  Measure      `csv:"TERPINOLENE" json:"terpinolene"`
	TransBFarnesene              Measure      `csv:"TRANS-B-FARNESENE" json:"trans_b_farnesene"`
	Valencene                    Measure      `csv:"VALENCENE" json:"valencene"`
	ACedrene                     Measure      `csv:"A-CEDRENE" json:"a_cedrene"`
	AFarnesene                   Measure      `csv:"A-FARNESENE" json:"a_farnesene"`
	BFarnesene                   Measure      `csv:"B-FARNESENE" json:"b_farnesene"`
	CisNerolidol                 Measure      `csv:"CIS-NEROLIDOL" json:"cis_nerolidol"`
	Fenchol                      Measure      `csv:"FENCHOL" json:"fenchol"`
	TransNerolidol               Measure      `csv:"TRANS-NEROLIDOL" json:"trans_nerolidol"`
	Market                       string       `csv:"Market" json:"market"`
	Chemotype                    string       `csv:"Chemotype" json:"chemotype"`
	ProcessingTechnique          string       `csv:"Processing Technique" json:"processing_technique"`
	SolventsUsed                 string       `csv:"Solvents Used" json:"solvents_used"`
	NationalDrugCode             string       `csv:"National Drug Code" json:"national_drug_code"`
}

///////////////////////////////////////////////////////////////////////////////

// FetchBrands fetches all the CT cannabis brands data from the CT API
func FetchBrands(appToken string, maxCacheAge time.Duration) ([]Brand, error) {
	// check cache
	if cacheBytes, err := data.CheckCacheFile(BRAND_JSON_FILENAME, maxCacheAge); err == nil {
		// Unmarshal the cache file
		var cacheBrands []Brand
		err := json.Unmarshal(cacheBytes, &cacheBrands)
		if err == nil {
			return cacheBrands, nil
		}
		// If unsuccessful, we will fetch the data from the API
	}

	// create a new cache file, with a preservation control bit
	cacheFile, err := data.MakeCacheFile(BRAND_JSON_FILENAME)
	if err != nil {
		return nil, fmt.Errorf("failed to create JSON cache file: %w", err)
	}
	deleteCacheFile := true
	defer func() {
		cacheFile.Close()
		if deleteCacheFile {
			os.Remove(cacheFile.Name())
		}
	}()
	cacheFile.WriteString("[")

	// prepare the URL
	brandsUrl, err := url.Parse(BrandsURL)
	if err != nil {
		return nil, err
	}

	var brands []Brand
	offset := 0
	firstLoop := true
	for {
		const batchLimit = 5000

		// compose the URL
		req, err := http.NewRequest("GET", brandsUrl.String(), nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		q.Add("$order", "registration_number")
		q.Add("$offset", strconv.Itoa(offset))
		q.Add("$limit", strconv.Itoa(batchLimit))
		if appToken != "" {
			q.Add("$$app_token", appToken)
		}
		req.URL.RawQuery = q.Encode()

		// do the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		badStatusCode := (resp.StatusCode != http.StatusOK)

		// Read the body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			if badStatusCode {
				return nil, fmt.Errorf("HTTP %d %s %s %w", resp.StatusCode, resp.Status, string(body), err)
			}
			return nil, err
		}
		if badStatusCode {
			return nil, fmt.Errorf("HTTP %d %s %s", resp.StatusCode, resp.Status, string(body))
		}

		// Unmarshal the response
		var brandsBatch []Brand
		if err := json.Unmarshal(body, &brandsBatch); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %w", err)
		}
		brands = append(brands, brandsBatch...)

		// Write to the cache
		if cacheFile != nil {
			// comma handling
			if !firstLoop {
				cacheFile.WriteString(",")
			}
			firstLoop = false
			// skip the first byte and last bytes, which are brackets
			body = bytes.TrimSpace(body)
			body = bytes.TrimPrefix(body, []byte("["))
			body = bytes.TrimSuffix(body, []byte("]"))
			cacheFile.Write(body)
		}

		// break or next batch
		if len(brandsBatch) < batchLimit {
			break
		}
		offset += batchLimit
	}

	// we made it, keep the cache file
	deleteCacheFile = false
	cacheFile.WriteString("]")

	// return the results
	return brands, nil
}

// CleanBrands modifies the passed brand slice in place, filtering out bad Brand samples using IsBrandErroneous().
// It returns the cleaned slice.
func CleanBrands(bs []Brand) []Brand {
	return slices.DeleteFunc(bs, func(b Brand) bool {
		return IsBrandErroneous(&b) // Delete if erroneous
	})
}

// IsBrandErroneous checks if the brand is erroneous, returning true if it is
func IsBrandErroneous(b *Brand) bool {
	if b.BrandName == "" {
		return true // We must have a name
	}
	if !b.TetrahydrocannabinolThc.IsValidPercent() || !b.TetrahydrocannabinolAcidThca.IsValidPercent() || !b.CannabidiolsCbd.IsValidPercent() || !b.CannabidiolAcidCbda.IsValidPercent() ||
		!b.APinene.IsValidPercent() || !b.BMyrcene.IsValidPercent() || !b.BCaryophyllene.IsValidPercent() || !b.BPinene.IsValidPercent() || !b.Limonene.IsValidPercent() || !b.Ocimene.IsValidPercent() ||
		!b.LinaloolLin.IsValidPercent() || !b.HumuleneHum.IsValidPercent() || !b.Cbg.IsValidPercent() || !b.CbgA.IsValidPercent() || !b.CannabavarinCbdv.IsValidPercent() || !b.CannabichromeneCbc.IsValidPercent() ||
		!b.CannbinolCbn.IsValidPercent() || !b.TetrahydrocannabivarinThcv.IsValidPercent() || !b.ABisabolol.IsValidPercent() || !b.APhellandrene.IsValidPercent() || !b.ATerpinene.IsValidPercent() || !b.BEudesmol.IsValidPercent() ||
		!b.BTerpinene.IsValidPercent() || !b.Fenchone.IsValidPercent() || !b.Pulegol.IsValidPercent() || !b.Borneol.IsValidPercent() || !b.Isopulegol.IsValidPercent() || !b.Carene.IsValidPercent() || !b.Camphene.IsValidPercent() ||
		!b.Camphor.IsValidPercent() || !b.CaryophylleneOxide.IsValidPercent() || !b.Cedrol.IsValidPercent() || !b.Eucalyptol.IsValidPercent() || !b.Geraniol.IsValidPercent() || !b.Guaiol.IsValidPercent() || !b.GeranylAcetate.IsValidPercent() ||
		!b.Isoborneol.IsValidPercent() || !b.Menthol.IsValidPercent() || !b.LFenchone.IsValidPercent() || !b.Nerol.IsValidPercent() || !b.Sabinene.IsValidPercent() || !b.Terpineol.IsValidPercent() || !b.Terpinolene.IsValidPercent() || !b.TransBFarnesene.IsValidPercent() ||
		!b.Valencene.IsValidPercent() || !b.ACedrene.IsValidPercent() || !b.AFarnesene.IsValidPercent() || !b.BFarnesene.IsValidPercent() || !b.CisNerolidol.IsValidPercent() || !b.Fenchol.IsValidPercent() || !b.TransNerolidol.IsValidPercent() {
		return true
	}
	return false
}

///////////////////////////////////////////////////////////////////////////////

// CSVHeaders returns the CSV headers for the Brand struct
func (b Brand) CSVHeaders() string {
	// Note newline in string
	return `"brand_name","dosage_form","branding_entity","product_image_url","product_image_desc","label_image_url","label_image_desc","lab_analysis_url","lab_analysis_desc","approval_date","registration_number","tetrahydrocannabinol_thc","tetrahydrocannabinol_acid_thca","cannabidiols_cbd","cannabidiol_acid_cbda","a_pinene","b_myrcene","b_caryophyllene","b_pinene","limonene","ocimene","linalool_lin","humulene_hum","cbg","cbg_a","cannabavarin_cbdv","cannabichromene_cbc","cannbinol_cbn","tetrahydrocannabivarin_thcv","a_bisabolol","a_phellandrene","a_terpinene","b_eudesmol","b_terpinene","fenchone","pulegol","borneol","isopulegol","carene","camphene","camphor","caryophyllene_oxide","cedrol","eucalyptol","geraniol","guaiol","geranyl_acetate","isoborneol","menthol","l_fenchone","nerol","sabinene","terpineol","terpinolene","trans_b_farnesene","valencene","a_cedrene","a_farnesene","b_farnesene","cis_nerolidol","fenchol","trans_nerolidol","market","chemotype","processing_technique","solvents_used","national_drug_code"
`
}

// CSVValue returns the CSV value for the Brand struct
func (b Brand) CSVValue() string {
	// Note newline in format string
	return fmt.Sprintf(`"%s","%s","%s","%s","%s","%s","%s","%s","%s","%s","%s",%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,"%s","%s","%s","%s","%s"
`,
		CSVString(b.BrandName), CSVString(b.DosageForm), CSVString(b.BrandingEntity),
		CSVString(b.ProductImage.URL), CSVString(b.ProductImage.Description),
		CSVString(b.LabelImage.URL), CSVString(b.LabelImage.Description),
		CSVString(b.LabAnalysis.URL), CSVString(b.LabAnalysis.Description),
		b.ApprovalDate.Format("2006-01-02T15:04:05-0700"), CSVString(b.RegistrationNumber),
		b.TetrahydrocannabinolThc.AsCSV(), b.TetrahydrocannabinolAcidThca.AsCSV(), b.CannabidiolsCbd.AsCSV(), b.CannabidiolAcidCbda.AsCSV(), b.APinene.AsCSV(),
		b.BMyrcene.AsCSV(), b.BCaryophyllene.AsCSV(), b.BPinene.AsCSV(), b.Limonene.AsCSV(), b.Ocimene.AsCSV(), b.LinaloolLin.AsCSV(), b.HumuleneHum.AsCSV(),
		b.Cbg.AsCSV(), b.CbgA.AsCSV(), b.CannabavarinCbdv.AsCSV(), b.CannabichromeneCbc.AsCSV(), b.CannbinolCbn.AsCSV(), b.TetrahydrocannabivarinThcv.AsCSV(),
		b.ABisabolol.AsCSV(), b.APhellandrene.AsCSV(), b.ATerpinene.AsCSV(), b.BEudesmol.AsCSV(), b.BTerpinene.AsCSV(), b.Fenchone.AsCSV(), b.Pulegol.AsCSV(),
		b.Borneol.AsCSV(), b.Isopulegol.AsCSV(), b.Carene.AsCSV(), b.Camphene.AsCSV(), b.Camphor.AsCSV(), b.CaryophylleneOxide.AsCSV(), b.Cedrol.AsCSV(),
		b.Eucalyptol.AsCSV(), b.Geraniol.AsCSV(), b.Guaiol.AsCSV(), b.GeranylAcetate.AsCSV(), b.Isoborneol.AsCSV(), b.Menthol.AsCSV(), b.LFenchone.AsCSV(),
		b.Nerol.AsCSV(), b.Sabinene.AsCSV(), b.Terpineol.AsCSV(), b.Terpinolene.AsCSV(), b.TransBFarnesene.AsCSV(), b.Valencene.AsCSV(), b.ACedrene.AsCSV(),
		b.AFarnesene.AsCSV(), b.BFarnesene.AsCSV(), b.CisNerolidol.AsCSV(), b.Fenchol.AsCSV(), b.TransNerolidol.AsCSV(),
		CSVString(b.Market), CSVString(b.Chemotype), CSVString(b.ProcessingTechnique), CSVString(b.SolventsUsed), CSVString(b.NationalDrugCode),
	)
}

// CSVString internally santizes a string for use in a CSV file field
func CSVString(str string) string {
	// Transform double quotes to single quotes
	return strings.Replace(str, `"`, `'`, -1)
}

///////////////////////////////////////////////////////////////////////////////

func DBInsertBrands(conn *sql.DB, brands []Brand) error {
	if len(brands) == 0 {
		return nil
	}

	sqlHeader := `INSERT INTO brands_us_ct (
brand_name,dosage_form,branding_entity,product_image_url,product_image_desc,label_image_url,
lavel_image_desc,lab_analysis_url,lab_analysis_desc,approval_date,registration_number,
tetrahydrocannabinol_thc,tetrahydrocannabinol_acid_thca,cannabidiols_cbd,cannabidiol_acid_cbda,
a_pinene,b_myrcene,b_caryophyllene,b_pinene,limonene,ocimene,linalool_lin,humulene_hum,cbg,
cbg_a,cannabavarin_cbdv,cannabichromene_cbc,cannbinol_cbn,tetrahydrocannabivarin_thcv,a_bisabolol,
a_phellandrene,a_terpinene,b_eudesmol,b_terpinene,fenchone,pulegol,borneol,isopulegol,carene,
camphene,camphor,caryophyllene_oxide,cedrol,eucalyptol,geraniol,guaiol,geranyl_acetate,isoborneol,
menthol,l_fenchone,nerol,sabinene,terpineol,terpinolene,trans_b_farnesene,valencene,a_cedrene,
a_farnesene,b_farnesene,cis_nerolidol,fenchol,trans_nerolidol,market,chemotype,processing_technique,
solvents_used,national_drug_code)
VALUES `
	sqlFooter := ` ON CONFLICT DO NOTHING;`
	sqlFormat := `('%s','%s','%s','%s','%s','%s','%s','%s','%s','%s','%s',%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,'%s','%s','%s','%s','%s')`

	// Build the query
	var sb strings.Builder
	sb.WriteString(sqlHeader)
	isFirst := true
	for _, b := range brands {
		if !isFirst {
			sb.WriteString(",\n")
		}
		isFirst = false
		sb.WriteString(fmt.Sprintf(sqlFormat,
			db.String(b.BrandName), db.String(b.DosageForm), db.String(b.BrandingEntity),
			db.String(b.ProductImage.URL), db.String(b.ProductImage.Description),
			db.String(b.LabelImage.URL), db.String(b.LabelImage.Description),
			db.String(b.LabAnalysis.URL), db.String(b.LabAnalysis.Description),
			b.ApprovalDate.Format("2006-01-02T15:04:05-0700"), db.String(b.RegistrationNumber),
			b.TetrahydrocannabinolThc.AsSQL(),
			b.TetrahydrocannabinolAcidThca.AsSQL(),
			b.CannabidiolsCbd.AsSQL(),
			b.CannabidiolAcidCbda.AsSQL(),
			b.APinene.AsSQL(),
			b.BMyrcene.AsSQL(),
			b.BCaryophyllene.AsSQL(),
			b.BPinene.AsSQL(),
			b.Limonene.AsSQL(),
			b.Ocimene.AsSQL(),
			b.LinaloolLin.AsSQL(),
			b.HumuleneHum.AsSQL(),
			b.Cbg.AsSQL(),
			b.CbgA.AsSQL(),
			b.CannabavarinCbdv.AsSQL(),
			b.CannabichromeneCbc.AsSQL(),
			b.CannbinolCbn.AsSQL(),
			b.TetrahydrocannabivarinThcv.AsSQL(),
			b.ABisabolol.AsSQL(),
			b.APhellandrene.AsSQL(),
			b.ATerpinene.AsSQL(),
			b.BEudesmol.AsSQL(),
			b.BTerpinene.AsSQL(),
			b.Fenchone.AsSQL(),
			b.Pulegol.AsSQL(),
			b.Borneol.AsSQL(),
			b.Isopulegol.AsSQL(),
			b.Carene.AsSQL(),
			b.Camphene.AsSQL(),
			b.Camphor.AsSQL(),
			b.CaryophylleneOxide.AsSQL(),
			b.Cedrol.AsSQL(),
			b.Eucalyptol.AsSQL(),
			b.Geraniol.AsSQL(),
			b.Guaiol.AsSQL(),
			b.GeranylAcetate.AsSQL(),
			b.Isoborneol.AsSQL(),
			b.Menthol.AsSQL(),
			b.LFenchone.AsSQL(),
			b.Nerol.AsSQL(),
			b.Sabinene.AsSQL(),
			b.Terpineol.AsSQL(),
			b.Terpinolene.AsSQL(),
			b.TransBFarnesene.AsSQL(),
			b.Valencene.AsSQL(),
			b.ACedrene.AsSQL(),
			b.AFarnesene.AsSQL(),
			b.BFarnesene.AsSQL(),
			b.CisNerolidol.AsSQL(),
			b.Fenchol.AsSQL(),
			b.TransNerolidol.AsSQL(),
			db.String(b.Market), db.String(b.Chemotype), db.String(b.ProcessingTechnique),
			db.String(b.SolventsUsed), db.String(b.NationalDrugCode)))
	}
	sb.WriteString(sqlFooter)

	// Execute the SQL statement
	_, err := conn.Exec(sb.String())
	if err != nil {
		return fmt.Errorf("db insert failed: %w", err)
	}
	return nil
}
