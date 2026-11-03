# Testing documentation index

Test strategy, guides, plans and result reports.

Back to the documentation index: [docs/README.md](../README.md). Descriptions are copied mechanically from each file (its first heading and first prose line) unless overridden in scripts/docs/index_description_overrides.tsv; non-Markdown artefacts are listed by name and type without being read. 25 entries.

| File | Description |
|---|---|
| [ANDROID_TESTING_GUIDE.md](ANDROID_TESTING_GUIDE.md) | Android Testing Guide - The Android and AndroidTV applications have comprehensive test suites with 184 tests covering ViewModels, Repositories, DAOs, and UI components. The test infrastructure... |
| [COMPREHENSIVE_TEST_VERIFICATION.md](COMPREHENSIVE_TEST_VERIFICATION.md) | 🎯 Catalogizer - Comprehensive Test Verification Report - This report provides comprehensive verification of the Catalogizer test infrastructure after the latest expansion phase. We have successfully added additional backend... |
| [CONTRACT_TESTING_GUIDE.md](CONTRACT_TESTING_GUIDE.md) | API Contract Testing Guide - Contract tests verify that API responses match the shapes expected by consumers, primarily the TypeScript API client (catalogizer-api-client). They protect against... |
| [E2E_TEST_PLAN.md](E2E_TEST_PLAN.md) | End-to-End (E2E) Test Plan - This document outlines the comprehensive E2E testing strategy for Catalogizer across all platforms: Web (Playwright), Android (Espresso/Maestro), and AndroidTV. |
| [FINAL_TEST_REPORT.md](FINAL_TEST_REPORT.md) | 🎉 Catalogizer Test Suite - Final Comprehensive Report - The Catalogizer test infrastructure has been successfully expanded to 469 comprehensive tests covering backend and frontend platforms. This represents a remarkable... |
| [FUZZ_TESTING_GUIDE.md](FUZZ_TESTING_GUIDE.md) | Fuzz Testing Guide - This project uses Go's built-in fuzz testing framework (testing.F) to find crashes, panics, and invariant violations by generating random inputs. |
| [PERFORMANCE_TESTING_GUIDE.md](PERFORMANCE_TESTING_GUIDE.md) | Performance Testing Guide - This guide covers the comprehensive performance testing suite for Catalogizer, including backend benchmarks, frontend performance analysis, and Core Web Vitals... |
| [PROTOCOL_TESTING_COMPLETE.md](PROTOCOL_TESTING_COMPLETE.md) | Catalogizer Protocol Testing - Complete Solution - This document outlines the comprehensive solution implemented for real-time file/directory rename detection across all supported protocols (SMB, FTP, NFS, WebDAV, Local)... |
| [STRESS_TEST_RESULTS.md](STRESS_TEST_RESULTS.md) | Stress Test Results & Baselines - Stress tests in catalog-api/tests/stress/ use Go's testing package with goroutine-based concurrency. |
| [TESTING.md](TESTING.md) | Testing Guide - Catalogizer has a comprehensive test suite covering backend (Go), frontend (React/TypeScript), and mobile (Android/Kotlin) platforms. This document provides guidance on... |
| [TESTING_REPORT.md](TESTING_REPORT.md) | Catalogizer Testing Report - This report summarizes the testing results for all modules in the Catalogizer project. The goal was to ensure all existing tests pass with 100% success rate. While... |
| [TEST_EXPANSION_PROGRESS.md](TEST_EXPANSION_PROGRESS.md) | Test Expansion Progress Tracker |
| [TEST_IMPLEMENTATION_SUMMARY.md](TEST_IMPLEMENTATION_SUMMARY.md) | Catalogizer Test Implementation Summary - Successfully implemented a comprehensive test suite across all Catalogizer platforms (backend, frontend, and mobile), established CI/CD automation with GitHub Actions,... |
| [TEST_RESULTS.md](TEST_RESULTS.md) | Test Execution Results |
| [TEST_RESULTS_SUMMARY.md](TEST_RESULTS_SUMMARY.md) | Catalogizer Project Test Results Summary - ✅ Completed Successfully |
| [TEST_STRATEGY.md](TEST_STRATEGY.md) | Test Strategy - This document describes the testing approach across all Catalogizer modules, including test categories, infrastructure, conventions, and how to write tests for each part... |
| [TEST_VALIDATION_SUMMARY.md](TEST_VALIDATION_SUMMARY.md) | Test Validation Summary - Successfully validated all integration and stress tests for production readiness. Identified and fixed critical database testing issues. All applicable tests now pass... |
| [adding-challenges.md](adding-challenges.md) | Adding New User Flow Challenges - All Catalogizer-specific user flow challenges live in catalog-api/challenges/userflow_*.go. These files import the generic pkg/userflow/ framework from the Challenges... |
| [challenge-map.md](challenge-map.md) | Challenge Map - Complete listing of all registered challenges organized by category, including dependency chains. |
| [container-setup.md](container-setup.md) | Container Setup for User Flow Testing - User flow testing runs in a containerized environment using Podman (not Docker). The test container stack is defined in docker-compose.test.yml at the project root and... |
| [overview.md](overview.md) | User Flow Testing Overview - The user flow testing suite provides exhaustive end-to-end validation of every Catalogizer application across all 6 platforms. Unlike the existing challenge suite... |
| [real-service-stack.md](real-service-stack.md) | Real-service test stack (rootless podman) - Constitution 11.4.27 allows mocks only in unit tests; every other test type runs against real services. This page describes the stack that provides them on a rootless... |
| [running-tests.md](running-tests.md) | Running User Flow Challenges - The simplest way to run user flow challenges is through the running catalog-api server. |
| [sftp-client.md](sftp-client.md) | Read-only SFTP client (submodules/filesystem/pkg/sftp) - Every sentence in the "Behaviour" and "Honest limits" sections is either pinned by a test that fails when the sentence stops being true (the test is named in |
| [troubleshooting.md](troubleshooting.md) | User Flow Testing Troubleshooting |
