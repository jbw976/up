// Copyright 2025 Upbound Inc.
// All rights reserved

package oidcauth

import (
	"context"
	"crypto/sha1" //nolint:gosec // AWS requires SHA1 for OIDC provider thumbprint
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

//go:embed help/aws.md
var awsHelp string

func (c *awsCmd) Help() string {
	return awsHelp
}

type awsCmd struct {
	Name   string `arg:"" help:"AWS IAM Role Name"`
	Policy string `arg:"" help:"AWS IAM Policy ARN"`

	OIDCProviderName   string `default:"proidc.upbound.io"                                                                                                             help:"AWS Identity Provider - OIDC Provider Name"`
	ProviderConfigName string `default:"default"                                                                                                                       help:"Provider AWS ProviderConfigName"`
	Sub                string `help:"Define the control plane name that the IAM Role trust policy will use in the 'sub' claim. Supports wildcards (using StringLike)."`
	Yes                bool   `default:"false"                                                                                                                         help:"When set to true, automatically accepts any confirmation prompts."`
	DryRun             bool   `help:"Print what changes would be made but do not take action."`

	ctp types.NamespacedName
}

// AfterApply sets default values in command after assignment and validation.
func (c *awsCmd) AfterApply(upCtx *upbound.Context) error {
	var ctp types.NamespacedName
	var isSpace bool
	if _, ctp, isSpace = upCtx.GetCurrentSpaceContextScope(); isSpace && ctp.Name == "" {
		return errors.New("no control plane context is defined. Use 'up ctx' to set an control plane inside a group context")
	}

	c.ctp = ctp
	return nil
}

// Run executes the AWS command.
func (c *awsCmd) Run(ctx context.Context, cl client.Client, upCtx *upbound.Context, printer upterm.Printer) error {
	if c.DryRun {
		return c.runDryRun(upCtx, printer)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to load AWS SDK config")
	}

	stsClient := sts.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return errors.Wrap(err, "failed to get caller identity")
	}

	// Prompt for user confirmation if the identity is available.
	if !c.Yes {
		confirmed, err := upterm.Confirm(fmt.Sprintf("Do you want to create the IAM Identity Provider OIDC and IAM Role using the following identity? %s", *identity.Arn), false)
		if err != nil {
			return errors.Wrap(err, "failed to get user confirmation")
		}

		if !confirmed {
			printer.PrintError("Operation cancelled by user; creation aborted.")
			return errors.New("operation cancelled by user")
		}
	}

	oidcProviderARN := ""
	if err = printer.WrapWithSuccessSpinner(
		"Find or Create IAM Identity Provider OIDC",
		func() error {
			oidcProviderARN, err = oidcProvider(ctx, iamClient, c.OIDCProviderName)
			if err != nil {
				return errors.Wrap(err, "failed to find IAM Identity Provider OIDC")
			}
			return nil
		},
	); err != nil {
		return err
	}

	var role *iam.CreateRoleOutput
	if err = printer.WrapWithSuccessSpinner(
		"Create IAM Role with IAM Policy",
		func() error {
			sub := c.ctp.Name
			if c.Sub != "" {
				sub = c.Sub
			}
			trustPolicy, err := c.buildTrustPolicy(oidcProviderARN, upCtx.Profile.Organization, sub)
			if err != nil {
				return errors.Wrap(err, "failed to build trust policy")
			}
			role, err = iamClient.CreateRole(ctx, &iam.CreateRoleInput{
				RoleName:                 aws.String(c.Name),
				AssumeRolePolicyDocument: aws.String(trustPolicy),
			})
			if err != nil {
				return errors.Wrap(err, "failed to create IAM role")
			}
			_, err = iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
				RoleName:  aws.String(c.Name),
				PolicyArn: aws.String(c.Policy),
			})
			if err != nil {
				return errors.Wrap(err, "failed to attach managed policy to role")
			}
			return nil
		},
	); err != nil {
		return err
	}

	// ToDo(haarchri): check if Family Provider AWS is installed
	providerConfig := c.buildProviderConfig(*role.Role.Arn)
	if err = printer.WrapWithSuccessSpinner(
		"Create ProviderConfig in ControlPlane",
		func() error {
			if err := cl.Patch(ctx, providerConfig, client.Apply, client.ForceOwnership, client.FieldOwner("up-ctp-auth-providerconfig")); err != nil {
				return errors.Wrap(err, "failed to create or update ProviderConfig")
			}
			return nil
		},
	); err != nil {
		return err
	}

	printer.PrintSuccess(fmt.Sprintf("OIDC Provider: %s", oidcProviderARN))
	printer.PrintSuccess(fmt.Sprintf("IAM Role: %s", *role.Role.Arn))
	printer.PrintSuccess(fmt.Sprintf("ProviderConfig: %s", providerConfig.GetName()))
	return nil
}

func (c *awsCmd) runDryRun(upCtx *upbound.Context, p upterm.Printer) error {
	p.PrintInfo("Dry-run mode: Showing CLI commands that would be executed")
	p.Println()

	// Build trust policy for display
	sub := c.ctp.Name
	if c.Sub != "" {
		sub = c.Sub
	}
	oidcProviderARN := fmt.Sprintf("arn:aws:iam::ACCOUNT_ID:oidc-provider/%s", c.OIDCProviderName)
	trustPolicy, err := c.buildTrustPolicy(oidcProviderARN, upCtx.Profile.Organization, sub)
	if err != nil {
		return errors.Wrap(err, "failed to build trust policy")
	}

	// Show OIDC provider commands
	p.Println("1. Check for existing OIDC provider:")
	p.Printfln("aws iam list-open-id-connect-providers")
	p.Printfln("aws iam get-open-id-connect-provider --open-id-connect-provider-arn arn:aws:iam::ACCOUNT_ID:oidc-provider/%s", c.OIDCProviderName)
	p.Println()

	p.Println("2. Create OIDC provider (if not exists):")
	p.Printfln("# Get thumbprint for the OIDC provider")
	p.Printfln("THUMBPRINT=$(echo | openssl s_client -servername %s -showcerts -connect %s:443 2>/dev/null | openssl x509 -fingerprint -sha1 -noout | sed 's/://g' | awk -F= '{print tolower($2)}')", c.OIDCProviderName, c.OIDCProviderName)
	p.Println()
	p.Printfln("aws iam create-open-id-connect-provider \\")
	p.Printfln("  --url https://%s \\", c.OIDCProviderName)
	p.Printfln("  --client-id-list sts.amazonaws.com \\")
	p.Printfln("  --thumbprint-list \"$THUMBPRINT\"")
	p.Println()

	p.Println("3. Create IAM role with trust policy:")
	p.Printfln("aws iam create-role \\")
	p.Printfln("  --role-name %s \\", c.Name)
	p.Printfln("  --assume-role-policy-document '%s'", trustPolicy)
	p.Println()

	p.Println("4. Attach policy to role:")
	p.Printfln("aws iam attach-role-policy \\")
	p.Printfln("  --role-name %s \\", c.Name)
	p.Printfln("  --policy-arn %s", c.Policy)
	p.Println()

	p.Println("5. Create ProviderConfig in ControlPlane:")
	// Build the ProviderConfig with a placeholder role ARN for dry-run
	roleARN := fmt.Sprintf("arn:aws:iam::ACCOUNT_ID:role/%s", c.Name)
	providerConfig := c.buildProviderConfig(roleARN)

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(providerConfig.Object)
	if err != nil {
		return errors.Wrap(err, "failed to marshal ProviderConfig to YAML")
	}

	p.Printfln("cat <<EOF | kubectl apply -f -\n%sEOF", string(yamlBytes))

	return nil
}

// oidcProvider looks for an OIDC provider with the given URL and returns its ARN or create the OIDC provider.
func oidcProvider(ctx context.Context, client *iam.Client, providerName string) (string, error) {
	listOutput, err := client.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return "", errors.Wrap(err, "failed to list OIDC providers")
	}

	for _, provider := range listOutput.OpenIDConnectProviderList {
		getOutput, err := client.GetOpenIDConnectProvider(ctx, &iam.GetOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: provider.Arn,
		})
		if err != nil || getOutput == nil {
			continue
		}
		if *getOutput.Url == providerName {
			return *provider.Arn, nil
		}
	}

	// Provider not found, get thumbprint and create one
	thumbprint, err := getSHA1Thumbprint(ctx, providerName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get thumbprint for %s", providerName)
	}

	createOutput, err := client.CreateOpenIDConnectProvider(ctx, &iam.CreateOpenIDConnectProviderInput{
		Url:            aws.String(fmt.Sprintf("https://%s", providerName)),
		ClientIDList:   []string{"sts.amazonaws.com"},
		ThumbprintList: []string{thumbprint},
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to create OIDC provider")
	}

	return *createOutput.OpenIDConnectProviderArn, nil
}

// buildProviderConfig creates a ProviderConfig manifest for AWS provider.
func (c *awsCmd) buildProviderConfig(roleARN string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "aws.upbound.io/v1beta1",
			"kind":       "ProviderConfig",
			"metadata": map[string]any{
				"name": c.ProviderConfigName,
			},
			"spec": map[string]any{
				"credentials": map[string]any{
					"source": "Upbound",
					"upbound": map[string]any{
						"webIdentity": map[string]any{
							"roleARN": roleARN,
						},
					},
				},
			},
		},
	}
}

// buildTrustPolicy creates a trust policy JSON string for the given OIDC provider ARN.
func (c *awsCmd) buildTrustPolicy(oidcProviderARN, org, controlplane string) (string, error) {
	conditionKey := "StringEquals"
	subValue := fmt.Sprintf("mcp:%s/%s:provider:provider-aws", org, controlplane)

	// Check if controlplane ends with "*"
	if strings.HasSuffix(controlplane, "*") {
		conditionKey = "StringLike"
		subValue = fmt.Sprintf("mcp:%s/%s:provider:provider-aws", org, controlplane)
	}

	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect": "Allow",
				"Principal": map[string]any{
					"Federated": oidcProviderARN,
				},
				"Action": "sts:AssumeRoleWithWebIdentity",
				"Condition": map[string]any{
					conditionKey: map[string]any{
						fmt.Sprintf("%s:sub", c.OIDCProviderName): subValue,
						fmt.Sprintf("%s:aud", c.OIDCProviderName): "sts.amazonaws.com",
					},
				},
			},
		},
	}

	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal trust policy")
	}

	return string(policyBytes), nil
}

func getSHA1Thumbprint(ctx context.Context, host string) (string, error) {
	d := &tls.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:443", host))
	if err != nil {
		return "", err
	}

	tconn, ok := conn.(*tls.Conn)
	if !ok {
		return "", errors.New("tls connection has wrong type")
	}
	certs := tconn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("no certificates found")
	}

	sha1sum := sha1.Sum(certs[0].Raw) //nolint:gosec // AWS requires SHA1 for OIDC provider thumbprint
	return hex.EncodeToString(sha1sum[:]), nil
}
