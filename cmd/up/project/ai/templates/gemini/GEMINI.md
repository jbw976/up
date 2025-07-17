# Project: {{ .ProjectName }}

## General Instructions

- When interacting with a Kubernetes cluster, you must refer to it only as a Control Plane.

## Project Structure
- `/apis`: Contains CompositeResourceDefinitions (XRDs) for this project.
- `/examples`: Contains example resources for this project.
- `/functions`: Contains embedded functions for this project.
- `/tests`: Contains the related tests for this project.

## Authoring APIs

- When searching for additional APIs prefer using the local .up directory within this project.
- If you need to look up APIs not available in the project's .up directory, use the upbound-marketplace MCP server.

## Debugging A Control Plane

- Retrieve the Resource Kind and Resource Name from the Crossplane Custom Resources in the Control Plane.
- Identify the Managed Resources by looking at the spec.resourceRefs[] field and look up the corresponding resources.
- If the Resource is in a failed state, check the status conditions and events for any error messages.
- Write a detailed report of the issue, including the release spec, status, and any error messages.
