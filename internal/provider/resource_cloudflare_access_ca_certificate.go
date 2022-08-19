package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceCloudflareAccessCACertificate() *schema.Resource {
	return &schema.Resource{
		Schema:        resourceCloudflareAccessCACertificateSchema(),
		CreateContext: resourceCloudflareAccessCACertificateCreate,
		ReadContext:   resourceCloudflareAccessCACertificateRead,
		UpdateContext: resourceCloudflareAccessCACertificateUpdate,
		DeleteContext: resourceCloudflareAccessCACertificateDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceCloudflareAccessCACertificateImport,
		},
		Description: heredoc.Doc(`
			Cloudflare Access can replace traditional SSH key models with
			short-lived certificates issued to your users based on the token
			generated by their Access login.
		`),
	}
}

func resourceCloudflareAccessCACertificateCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*cloudflare.API)

	identifier, err := initIdentifier(d)
	if err != nil {
		return diag.FromErr(err)
	}

	var accessCACert cloudflare.AccessCACertificate
	if identifier.Type == AccountType {
		accessCACert, err = client.CreateAccessCACertificate(ctx, identifier.Value, d.Get("application_id").(string))
	} else {
		accessCACert, err = client.CreateZoneLevelAccessCACertificate(ctx, identifier.Value, d.Get("application_id").(string))
	}
	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating Access CA Certificate for %s %q: %w", identifier.Type, identifier.Value, err))
	}

	d.SetId(accessCACert.ID)

	return resourceCloudflareAccessCACertificateRead(ctx, d, meta)
}

func resourceCloudflareAccessCACertificateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*cloudflare.API)
	applicationID := d.Get("application_id").(string)
	identifier, err := initIdentifier(d)
	if err != nil {
		return diag.FromErr(err)
	}

	var accessCACert cloudflare.AccessCACertificate
	if identifier.Type == AccountType {
		accessCACert, err = client.AccessCACertificate(ctx, identifier.Value, applicationID)
	} else {
		accessCACert, err = client.ZoneLevelAccessCACertificate(ctx, identifier.Value, applicationID)
	}

	if err != nil {
		var notFoundError *cloudflare.NotFoundError
		if errors.As(err, &notFoundError) {
			tflog.Info(ctx, fmt.Sprintf("Access CA Certificate %s no longer exists", d.Id()))
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("error finding Access CA Certificate %q: %w", d.Id(), err))
	}

	d.Set("aud", accessCACert.Aud)
	d.Set("public_key", accessCACert.PublicKey)

	return nil
}

func resourceCloudflareAccessCACertificateUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceCloudflareAccessCACertificateDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*cloudflare.API)
	applicationID := d.Get("application_id").(string)

	tflog.Debug(ctx, fmt.Sprintf("Deleting Cloudflare CA Certificate using ID: %s", d.Id()))

	identifier, err := initIdentifier(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if identifier.Type == AccountType {
		err = client.DeleteAccessCACertificate(ctx, identifier.Value, applicationID)
	} else {
		err = client.DeleteZoneLevelAccessCACertificate(ctx, identifier.Value, applicationID)
	}

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return nil
}

func resourceCloudflareAccessCACertificateImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	attributes := strings.SplitN(d.Id(), "/", 3)

	if len(attributes) != 3 {
		return nil, fmt.Errorf("invalid id (\"%s\") specified, should be in format \"account/accountID/accessCACertificateID\" or \"zone/zoneID/accessCACertificateID\"", d.Id())
	}

	identifierType, identifierID, accessCACertificateID := attributes[0], attributes[1], attributes[2]

	if AccessIdentifierType(identifierType) != AccountType && AccessIdentifierType(identifierType) != ZoneType {
		return nil, fmt.Errorf("invalid id (\"%s\") specified, should be in format \"account/accountID/accessCACertificateID\" or \"zone/zoneID/accessCACertificateID\"", d.Id())
	}

	tflog.Debug(ctx, fmt.Sprintf("Importing Cloudflare Access CA Certificate: id %s for %s %s", accessCACertificateID, identifierType, identifierID))

	//lintignore:R001
	d.Set(fmt.Sprintf("%s_id", identifierType), identifierID)
	d.SetId(accessCACertificateID)

	readErr := resourceCloudflareAccessCACertificateRead(ctx, d, meta)
	if readErr != nil {
		return nil, errors.New("failed to read Access CA Certificate state")
	}

	return []*schema.ResourceData{d}, nil
}
