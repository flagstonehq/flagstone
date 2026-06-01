# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]


## [v0.1.0] - 2026-06-01

[v0.1.0]: https://github.com/thomas-vilte/flagstone/compare/v0.0.0...v0.1.0

We are proud to introduce the initial release of Flagstone, a powerful feature management platform. We have focused on building a secure, multi-tenant foundation that includes robust flag evaluation, comprehensive security features, and a modern user interface to help you control your software delivery.

### 🚩 Feature Management

- Implemented core feature flag evaluation logic with expanded operator support.
- Added full CRUD operations for managing projects, environments, flags, and segments.
- Introduced flag management capabilities including archiving, toggling, and detailed environment-specific configurations.
- Added a loading skeleton to the flags page to improve the perceived performance during data fetching.

### 🔒 Security & Authentication

- Built a comprehensive authentication flow featuring JWT integration, refresh tokens, and token rotation.
- Implemented advanced security measures including account lockout, token reuse prevention, and tenant enumeration protection.
- Added API key management to allow secure programmatic access to the platform.
- Integrated cryptographic utilities to ensure secure data handling across the system.

### 🎨 User Interface

- Scaffolded the frontend application using Next.js and Shadcn/ui for a modern, responsive interface.
- Consolidated project management workflows to streamline how users create and organize their work.
- Improved login error handling and overall authentication UX for a smoother onboarding experience.

### 🛡️ Stability & Operations

- Implemented a global audit log API and query endpoints to track all administrative actions.
- Added robust system startup procedures with integrated health and readiness checks.
- Integrated PostgreSQL and Redis client libraries to provide a high-performance persistence layer.
- Enabled transaction support via a unified Querier interface to ensure data integrity.

### 🔧 Developer Experience

- Enhanced the testing infrastructure using @testing-library/dom and comprehensive test suites for login and flag management.
- Consolidated domain stores and persistence layers to simplify the codebase and improve maintainability.
- Standardized request handling and middleware for consistent API behavior.

