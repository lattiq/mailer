# OTP Email Example - Using Templates with AWS SES

This example demonstrates how to send OTP (One-Time Password) emails using templates with the mailer library and AWS SES provider.

## Template-based Email Sending

Instead of generating HTML content directly in your code, you can use templates to separate presentation from logic.

### Directory Structure

```
examples/otp/
├── main.go              # Example using AWS SES with templates
├── templates/           # Template files
│   ├── otp.html.html   # HTML template for OTP emails (double extension)
│   └── otp.text.text   # Text template for OTP emails (double extension)
└── README.md           # This file
```

### How it Works

1. **Configuration**: Enable templates and configure AWS SES provider:

   ```go
   config := mailer.DefaultConfig()
   config.Templates.Enabled = true
   config.Templates.Directory = "templates" // Relative to current working directory
   config.Templates.Extension = []string{".html", ".text"}

   client, err := mailer.New(config, mailer.WithAWSSES("ap-south-1"))
   if err != nil {
       log.Fatal(err)
   }
   defer client.Close()
   ```

2. **Template Files**: Create template files with Go template syntax:

   - `otp.html.html` - HTML version of the email (note the double extension)
   - `otp.text.text` - Plain text version of the email (note the double extension)

   **Important**: Template files must use double extensions (e.g., `name.html.html`, `name.text.text`) so that when the file extension is removed during loading, the templates are registered with names like `name.html` and `name.text`, which is what `SendTemplate` expects.

3. **Template Data**: Define a struct with the data you want to pass to templates:

   ```go
   type OTPData struct {
       UserName       string
       OTP            string
       ExpiryTime     string
       ExpiryDuration string
       CompanyName    string
       CompanyLogo    string
       SupportEmail   string
       AppName        string
   }
   ```

4. **Send with Template**: Use `SendTemplate` instead of `Send`:

   ```go
   templateRequest := &mailer.TemplateRequest{
       Template: "otp", // Template name (without extension)
       From:     mailer.Address{Email: "otp@lattiq.com", Name: "LattIQ"},
       To:       []mailer.Address{{Email: "user@lattiq.com", Name: "User"}},
       Subject:  fmt.Sprintf("Your %s login code: %s", otpData.AppName, otpData.OTP),
       Data:     otpData,
       Headers: map[string]string{
           "X-Category": "authentication",
           "X-OTP-Type": "login",
       },
   }

   err := client.SendTemplate(context.Background(), templateRequest)
   ```

### Template Syntax

Templates use Go's `html/template` and `text/template` syntax:

- `{{.FieldName}}` - Insert field value
- `{{if .Field}}...{{end}}` - Conditional rendering
- `{{range .Items}}...{{end}}` - Loop over items
- `{{now | formatTime "2006"}}` - Use built-in functions

### Built-in Template Functions

The mailer provides several built-in functions:

- `upper`, `lower`, `title` - String case conversion
- `trim` - Remove whitespace
- `now` - Current time
- `formatTime` - Format time with layout
- `add`, `sub`, `mul`, `div` - Math operations
- `eq`, `ne`, `lt`, `gt` - Comparisons

### Benefits of Using Templates

1. **Separation of Concerns**: Keep presentation separate from business logic
2. **Maintainability**: Easier to update email designs without code changes
3. **Consistency**: Reuse templates across different parts of your application
4. **Designer Friendly**: Non-programmers can modify email designs
5. **Multi-format**: Send both HTML and text versions using separate template files

### Prerequisites

Before running this example, ensure you have:

1. **AWS SES Setup**:

   - SES configured in the `ap-south-1` region
   - Email addresses verified in SES (both sender and recipient)
   - SES moved out of sandbox mode (if sending to unverified addresses)

2. **AWS Credentials**: Configure credentials via one of these methods:
   - AWS CLI: `aws configure`
   - Environment variables: `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
   - IAM role (if running on EC2)

### Running the Example

```bash
cd examples/otp
go run main.go
```
