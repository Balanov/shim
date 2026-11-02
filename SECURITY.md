# Security Policy

## Reporting

Please report vulnerabilities privately through GitHub Security Advisories for this repository. Include the affected version/commit, operating system, reproduction steps, impact, and a minimal proof of concept. Do not include real credentials or private user data.

## Threat model and current limits

Shim connects model output to local tools, so prompt injection is a meaningful boundary. Repository text, skills, web content, and MCP output are untrusted.

Current mitigations:

- default confirmation for writes, shell, and MCP;
- read-only mode;
- workspace lexical and symlink path checks;
- allowlisted and confirmed natural-language management commands;
- no credential storage in the JSON config;
- bounded tool output and network/process timeouts.

Shim is **not an OS sandbox**. `auto` mode, shell access, and MCP servers can affect anything available to your user account. Run untrusted workloads in a dedicated container, VM, or locked-down OS account. Review third-party skills and MCP servers before connecting them.

Security fixes take priority over feature work. Supported versions are the latest `main` commit and the newest tagged release.
