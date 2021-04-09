package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"4d63.com/tz"
	"github.com/joho/godotenv"
	"github.com/ovh/go-ovh/ovh"
	"github.com/pkg/browser"
	"github.com/urfave/cli"
)

func createAPIToken() error {
	err := browser.OpenURL("https://eu.api.ovh.com/createToken/?GET=/me/bill&GET=/me/bill/*&GET=/me/deposit&GET=/me/deposit/*&GET=/me/order&GET=/me/order/*&GET=/me/refund&GET=/me/refund/*")
	return err
}

type priceStruct struct {
	Value    float64 `json:"value"`
	Text     string  `json:"text"`
	Currency string  `json:"currencyCode"`
}

type billResponse struct {
	BillID       string      `json:"billId"`
	PdfURL       string      `json:"pdfUrl"`
	Date         string      `json:"date"`
	Price        priceStruct `json:"priceWithoutTax"`
	Tax          priceStruct `json:"tax"`
	PriceWithTax priceStruct `json:"priceWithTax"`
}

type refundResponse struct {
	RefundID     string      `json:"refundId"`
	PdfURL       string      `json:"pdfUrl"`
	Date         string      `json:"date"`
	Price        priceStruct `json:"priceWithoutTax"`
	Tax          priceStruct `json:"tax"`
	PriceWithTax priceStruct `json:"priceWithTax"`
}

type depositResponse struct {
	DepositID string      `json:"depositId"`
	PdfURL    string      `json:"pdfUrl"`
	Date      string      `json:"date"`
	Price     priceStruct `json:"amount"`
}

func downloadInvoices(c *cli.Context) error {
	ovhEndpoint := c.String("ovh-endpoint")
	ovhAk := c.String("ovh-ak")
	ovhAs := c.String("ovh-as")
	ovhCk := c.String("ovh-ck")
	dir := c.String("dir")
	year := c.String("year")
	month := c.String("month")

	cwd, _ := os.Getwd()
	_, error := os.Stat(cwd + "/" + dir)
	if error != nil {
		return errors.New("Folder " + cwd + "/" + dir + " does not exist")
	}

	_, error = os.Stat(cwd + "/" + dir + "/" + year + "/" + month)
	if error != nil {
		if os.IsNotExist(error) {
			error := os.MkdirAll(cwd+"/"+dir+"/"+year+"/"+month, 0777)
			if error != nil {

			}
		}
	}
	location, _ := tz.LoadLocation("Europe/Paris")
	firstday, _ := time.ParseInLocation("2006-01-02", year+"-"+month+"-01", location)
	duration, _ := time.ParseDuration("-1s")
	lastday := firstday.AddDate(0, 1, 0).Add(duration)

	ovhapi, _ := ovh.NewClient(
		ovhEndpoint,
		ovhAk,
		ovhAs,
		ovhCk,
	)

	billDepositMap := map[string]string{}

	deposits := []string{}

	err := ovhapi.Get("/me/deposit?date.from="+url.QueryEscape(firstday.Format("2006-01-02T15:04:05Z07:00"))+"&date.to="+url.QueryEscape(lastday.Format("2006-01-02T15:04:05Z07:00")), &deposits)
	if err != nil {
		fmt.Println(err)
	} else {

		depositsCSV := [][]string{}

		row := []string{"Prélèvement", "Date", "Prix TTC", "Factures"}
		depositsCSV = append(depositsCSV, row)

		for _, depositID := range deposits {
			deposit := depositResponse{}
			err := ovhapi.Get("/me/deposit/"+depositID, &deposit)
			if err == nil {
				file := cwd + "/" + dir + "/" + year + "/" + month + "/" + depositID + ".pdf"
				if _, err := os.Stat(file); err != nil {
					if os.IsNotExist(err) {
						resp, err := http.Get(deposit.PdfURL)
						if err == nil {
							f, err := os.Create(file)
							if err == nil {
								io.Copy(f, resp.Body)
								f.Close()
							}
							resp.Body.Close()
						}
					}
				}

				depositBills := []string{}
				err := ovhapi.Get("/me/deposit/"+depositID+"/paidBills", &depositBills)
				if err != nil {
					fmt.Println(err)
				}

				for _, billID := range depositBills {
					billDepositMap[billID] = depositID
				}

				row = []string{
					deposit.DepositID,
					deposit.Date[8:10] + "/" + deposit.Date[5:7] + "/" + deposit.Date[:4],
					strings.Replace(fmt.Sprintf("%.2f", deposit.Price.Value), ".", ",", -1),
					strings.Join(depositBills, ", "),
				}
				depositsCSV = append(depositsCSV, row)

			}

		}
		fileCsv, _ := os.Create(cwd + "/" + dir + "/" + year + "/" + month + "/deposits.csv")
		w := csv.NewWriter(fileCsv)
		w.Comma = ';'
		w.UseCRLF = false
		w.WriteAll(depositsCSV)
		fileCsv.Close()
	}

	bills := []string{}
	billsCSV := [][]string{}

	row := []string{"Facture", "Date", "Prix HT", "TVA", "Prix TTC", "Prélèvement"}
	billsCSV = append(billsCSV, row)

	totalPrice := 0.0
	totalPriceWithTax := 0.0
	totalTax := 0.0

	err = ovhapi.Get("/me/bill?date.from="+url.QueryEscape(firstday.Format("2006-01-02T15:04:05Z07:00"))+"&date.to="+url.QueryEscape(lastday.Format("2006-01-02T15:04:05Z07:00")), &bills)
	if err != nil {
		fmt.Println(err)
	} else {
		for _, billID := range bills {
			bill := billResponse{}
			err := ovhapi.Get("/me/bill/"+billID, &bill)
			if err == nil {
				totalPrice += bill.Price.Value
				totalPriceWithTax += bill.PriceWithTax.Value
				totalTax += bill.Tax.Value
				row = []string{
					bill.BillID,
					bill.Date[8:10] + "/" + bill.Date[5:7] + "/" + bill.Date[:4],
					strings.Replace(fmt.Sprintf("%.2f", bill.Price.Value), ".", ",", -1),
					strings.Replace(fmt.Sprintf("%.2f", bill.Tax.Value), ".", ",", -1),
					strings.Replace(fmt.Sprintf("%.2f", bill.PriceWithTax.Value), ".", ",", -1),
					billDepositMap[billID],
				}
				billsCSV = append(billsCSV, row)
				file := cwd + "/" + dir + "/" + year + "/" + month + "/" + billID + ".pdf"
				if _, err := os.Stat(file); err != nil {
					if os.IsNotExist(err) {
						resp, err := http.Get(bill.PdfURL)
						if err == nil {
							f, err := os.Create(file)
							if err == nil {
								io.Copy(f, resp.Body)
								f.Close()
							}
							resp.Body.Close()
						}
					}
				}
			}
		}
	}

	refunds := []string{}
	err = ovhapi.Get("/me/refund?date.from="+url.QueryEscape(firstday.Format("2006-01-02T15:04:05Z07:00"))+"&date.to="+url.QueryEscape(lastday.Format("2006-01-02T15:04:05Z07:00")), &refunds)
	if err != nil {
		fmt.Println(err)
	} else {
		for _, refundID := range refunds {
			refund := refundResponse{}
			err := ovhapi.Get("/me/refund/"+refundID, &refund)
			if err == nil {
				totalPrice += refund.Price.Value
				totalPriceWithTax += refund.PriceWithTax.Value
				totalTax += refund.Tax.Value
				row = []string{
					refund.RefundID,
					refund.Date[8:10] + "/" + refund.Date[5:7] + "/" + refund.Date[:4],
					strings.Replace(fmt.Sprintf("%.2f", refund.Price.Value), ".", ",", -1),
					strings.Replace(fmt.Sprintf("%.2f", refund.Tax.Value), ".", ",", -1),
					strings.Replace(fmt.Sprintf("%.2f", refund.PriceWithTax.Value), ".", ",", -1),
					"",
				}
				billsCSV = append(billsCSV, row)
				file := cwd + "/" + dir + "/" + year + "/" + month + "/" + refundID + ".pdf"
				if _, err := os.Stat(file); err != nil {
					if os.IsNotExist(err) {
						resp, err := http.Get(refund.PdfURL)
						if err == nil {
							f, err := os.Create(file)
							if err == nil {
								io.Copy(f, resp.Body)
								f.Close()
							}
							resp.Body.Close()
						}
					}
				}
			}
		}
	}

	row = []string{
		"Totaux",
		"",
		strings.Replace(fmt.Sprintf("%.2f", totalPrice), ".", ",", -1),
		strings.Replace(fmt.Sprintf("%.2f", totalTax), ".", ",", -1),
		strings.Replace(fmt.Sprintf("%.2f", totalPriceWithTax), ".", ",", -1),
		"",
	}
	billsCSV = append(billsCSV, row)

	fileCsv, _ := os.Create(cwd + "/" + dir + "/" + year + "/" + month + "/bills.csv")
	w := csv.NewWriter(fileCsv)
	w.Comma = ';'
	w.UseCRLF = false
	w.WriteAll(billsCSV)
	fileCsv.Close()

	return nil
}

func main() {

	dotEnvError := godotenv.Load()
	if dotEnvError != nil {
	}

	app := cli.NewApp()
	app.Name = "OVH download invoice"
	app.Author = "Julien Issler"
	app.Email = "julien@issler.net"
	app.Version = "0.3.0"

	now := time.Now()

	app.Commands = []cli.Command{
		{
			Name: "init",
			Action: func(c *cli.Context) error {
				err := createAPIToken()
				return err
			},
		},
		{
			Name: "download",
			Action: func(c *cli.Context) error {
				err := downloadInvoices(c)
				return err
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "ovh-endpoint",
					Value:  "ovh-eu",
					Usage:  "OVH API endpoint",
					EnvVar: "OVH_ENDPOINT",
				},
				cli.StringFlag{
					Name:   "ovh-ak",
					Value:  "",
					Usage:  "OVH API Application Key",
					EnvVar: "OVH_AK",
				},
				cli.StringFlag{
					Name:   "ovh-as",
					Value:  "",
					Usage:  "OVH API Application Secret",
					EnvVar: "OVH_AS",
				},
				cli.StringFlag{
					Name:   "ovh-ck",
					Value:  "",
					Usage:  "OVH API Consumer Key",
					EnvVar: "OVH_CK",
				},
				cli.StringFlag{
					Name:   "dir",
					Value:  "invoices",
					Usage:  "directory where invoices will be stored, relative to current directory",
					EnvVar: "INVOICE_DIR",
				},
				cli.StringFlag{
					Name:  "year",
					Value: now.Format("2006"),
					Usage: "From date",
				},
				cli.StringFlag{
					Name:  "month",
					Value: now.Format("01"),
					Usage: "To date",
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
