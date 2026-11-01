package contracts

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"

	"doula-cloud/api/internal/pdftext"
)

// #2359: the signed Contract PDF prints the prose and the signature line's
// full legal name in any script as the characters typed, through
// pdffont's embedded faces.
func TestRenderContractPDF_PrintsEveryScriptAsTyped(t *testing.T) {
	cases := []struct{ script, name, prose string }{
		{"Vietnamese", "Nguyễn Thị Hương", "Hợp đồng này giữa Sài Gòn Doulas và Nguyễn Thị Hương."},
		{"Polish", "Łucja Żółkiewska-Wróbel", "Umowa między Pomoc Porodowa a Łucja Żółkiewska-Wróbel."},
		{"Arabic", "سارة الحداد", "هذه الاتفاقية بين دولا المدينة وسارة الحداد."},
		{"Chinese", "王小明", "本协议由北京导乐与王小明签订。"},
		{"Japanese", "山田さくら", "この契約は山田さくらとの間で結ばれます。"},
		{"Devanagari", "प्रिया शर्मा", "यह समझौता जन्म सहयोग और प्रिया शर्मा के बीच है।"},
		{"Cyrillic", "Анна Петрова", "Договор между Доула Москва и Анна Петрова."},
		{"a combining mark", "Zoë Brontë", "This agreement is between Rooted Birth Collective and Zoë Brontë."},
	}
	signedAt := time.Date(2026, time.October, 27, 14, 5, 0, 0, time.UTC)
	for _, tc := range cases {
		t.Run(tc.script, func(t *testing.T) {
			out, err := renderContractPDF(tc.prose, signature{fullLegalName: tc.name, signedAt: signedAt})
			if err != nil {
				t.Fatalf("renderContractPDF: %v", err)
			}
			got := strings.Split(pdftext.Text(out), "\n")
			want := []string{
				tc.prose,
				"Signed electronically by: " + tc.name,
				"Signed on: Oct 27, 2026, 2:05pm UTC",
				"Confirmed: " + signConsentLine,
			}
			if strings.Join(got, "|") != norm.NFC.String(strings.Join(want, "|")) {
				t.Fatalf("PDF text = %q, want %q", got, want)
			}
		})
	}
}
