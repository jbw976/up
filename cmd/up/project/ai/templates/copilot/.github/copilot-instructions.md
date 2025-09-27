# Project: {{ .ProjectName }}

## Project Overview

This is a Crossplane project that extends Kubernetes with infrastructure orchestration capabilities. When interacting with a Kubernetes cluster, you must refer to it only as a Control Plane.

## Folder Structure

- `/apis`: Contains CompositeResourceDefinitions (XRDs) for this project
- `/examples`: Contains example resources for this project
- `/functions`: Contains embedded functions for this project
- `/tests`: Contains the related tests for this project
- `/.up`: Local API directory with cached resources

## Libraries and Frameworks

- Crossplane for infrastructure orchestration
- Kubernetes APIs for resource definitions
- Go for function development (if applicable)

## Coding Standards

- Use YAML for Kubernetes resource definitions
- Follow Crossplane naming conventions for XRDs and Compositions
- Use semantic versioning for API versions
- Include proper metadata labels and annotations

## Debugging Guidelines

When debugging Control Plane issues:

- Retrieve the Resource Kind and Resource Name from the Crossplane Custom Resources
- Identify Managed Resources by looking at the `spec.resourceRefs[]` field and look up the corresponding resources
- If the Resource is in a failed state, check the status conditions and events for any error messages
- Write a detailed report of the issue, including the release spec, status, and any error messages

## API Development

- When searching for additional APIs prefer using the local `.up` directory within this project
- If you need to look up APIs not available in the project's `.up` directory, reference external Crossplane documentation
- Always validate XRD schemas before implementation
- Include comprehensive examples for each API