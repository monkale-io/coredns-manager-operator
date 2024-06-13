# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.3] - 2024-06-13
### Fixed
- 
- Bug Fix: Resolve validation of the spec.primaryNS.hostname allowing the creation of more complex hostnames required for certain DNS setups.
  https://github.com/monkale-io/coredns-manager-operator/issues/7
  
## [1.0.2] - 2024-06-05
### Fixed
- Bug Fix: Resolve Zone File Mount Issues with Dashed Domain Names
  - Fixed a bug where domain names containing dashes were causing issues with zone file mounting.

## [1.0.1] - 2024-06-04
### Added
- Initial release with all functionalities.
  - Basic functionality for managing DNS records.
  - Integration with CoreDNS.
  - Support for Kubernetes custom resources `dnsrecord` and `dnszone`.
