package south2md

import "testing"

func TestExtractMainPostReturnsAuthErrorForCloudflarePage(t *testing.T) {
	parser := NewPostParser(&HTMLSelectors{
		PostTable: "table.js-post",
	})

	html := `<!doctype html>
<html>
<head><title>Just a moment...</title></head>
<body>
<h1>Checking your browser before accessing the site.</h1>
<div>Cloudflare Ray ID: 1234567890</div>
</body>
</html>`

	if err := parser.LoadFromString(html); err != nil {
		t.Fatalf("load html failed: %v", err)
	}

	_, err := parser.ExtractMainPost()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Type != AuthError {
		t.Fatalf("expected AuthError, got %s", appErr.Type)
	}
}

func TestExtractMainPostReturnsValidationErrorForGenericPage(t *testing.T) {
	parser := NewPostParser(&HTMLSelectors{
		PostTable: "table.js-post",
	})

	html := `<!doctype html><html><head><title>Normal page</title></head><body>hello</body></html>`
	if err := parser.LoadFromString(html); err != nil {
		t.Fatalf("load html failed: %v", err)
	}

	_, err := parser.ExtractMainPost()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Type != ValidationError {
		t.Fatalf("expected ValidationError, got %s", appErr.Type)
	}
}
