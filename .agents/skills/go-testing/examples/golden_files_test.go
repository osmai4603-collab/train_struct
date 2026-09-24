package examples_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// راية تحديث الملفات المرجعية عبر سطر الأوامر (go test -update)
var updateGolden = flag.Bool("update", false, "update golden files with latest outputs")

// RenderSummaryTemplate ينتج تقريراً نصياً معقداً
func RenderSummaryTemplate(serviceName string, uptimePercent float64, totalRequests int) string {
	return fmt.Sprintf("=== SERVICE REPORT ===\nService: %s\nUptime: %.2f%%\nTotal Requests: %d\nStatus: OPERATIONAL\n",
		serviceName, uptimePercent, totalRequests)
}

func TestRenderSummaryTemplate_Golden(t *testing.T) {
	got := RenderSummaryTemplate("PaymentGateway", 99.95, 150042)

	goldenPath := filepath.Join("testdata", "summary_report.golden")

	// إذا طُلب التحديث عبر الراية، يتم حفظ المخرجات الحالية كمرجع
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatalf("failed to create testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
	}

	// قراءة الملف الذهبي المعتمد
	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden file %q does not exist. Run 'go test -update' to create it.", goldenPath)
		}
		t.Fatalf("failed to read golden file: %v", err)
	}

	// المقارنة بواسطة go-cmp
	if diff := cmp.Diff(string(wantBytes), got); diff != "" {
		t.Errorf("RenderSummaryTemplate() mismatch against golden file (-want +got):\n%s", diff)
	}
}
