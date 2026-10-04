# Forge PDF Export Specification

## 1. Overview

Forge supports generating CCF CSP-compliant contest problem papers. The visual standard and layout directly follow the official CSP-J/S second-round problem format (as exemplified in `C:\Users\Administrator\Desktop\CSP集训\第二天\`).

The export endpoint is:
```text
GET /api/contests/{contestID}/problems/{problemID}/export?locale=zh-CN
GET /api/contests/{contestID}/export?locale=zh-CN (all problems merged)
```

---

## 2. Layout & Typography Standards

### 2.1 Page Setup
- **Paper Size:** Standard ISO A4 (`@page { size: A4; margin: 16mm 20mm 17mm; }`).
- **Typography:**
  - Standard body text: SimSun (`simsun.ttc`, 宋体), 10.5pt, line-height 1.6.
  - Code & monospace: Consolas (`consola.ttf`), 9.5pt.
  - Section emphasis & explanations: KaiTi (`simkai.ttf`, 楷体).

### 2.2 Cover Page Structure (CCF CSP Standard)
1. **Title Header:**
   - e.g. "2026 CCF 非专业级软件能力认证"
   - "CSP-J/S 2026 第二轮认证" / "入门级 (CSP-J) / 提高级 (CSP-S)"
2. **Metadata Matrix Table:**
   - Problem Name (中文名称)
   - Problem Code / Directory (英文代号)
   - Executable Name
   - Input / Output File Names
   - Time Limit (e.g. 1.0s)
   - Memory Limit (e.g. 512 MiB)
   - Number of Testcases (e.g. 10-20)
3. **Compiler Directives:**
   - `-O2 -std=c++14 -static`
4. **Candidate Code of Conduct:**
   - Standard 8-rule compliance banner.

### 2.3 Problem Statement Page Structure
1. **Running Header:**
   - Left: Contest designation.
   - Right: Problem Chinese name and English code.
   - Bottom underline separator.
2. **Body Modules:**
   - `【题目描述】`
   - `【输入格式】`
   - `【输出格式】`
   - `【样例 N 输入】` & `【样例 N 输出】` (styled with line numbering and `#405dff` border).
   - `【样例解释】`
   - `【数据范围与提示】`
   - `【测试点数据范围】` (standard 4-column distribution table).

---

## 3. Generation Approaches

Forge provides a tiered architecture for PDF generation:

1. **Tier 1 (Zero-Dependency Browser Print):**
   - The export endpoint serves self-contained HTML with print CSS.
   - Teachers or students click **Export PDF** (or press `Ctrl+P`) and choose "Save as PDF" with background graphics enabled.
   - Zero external software required.

2. **Tier 2 (Chrome DevTools Protocol - Edge/Chromium Headless):**
   - Go controls the system's installed Edge/Chrome via CDP (`--headless --print-to-pdf`).
   - Automatically generates the PDF without requiring Node.js or Python.

3. **Tier 3 (External Playwright + PyMuPDF Pipeline):**
   - Compatible with the pipeline in `C:\Users\Administrator\Desktop\CSP集训\第二天`:
     - Playwright renders `build_mock_pdf.html` to raw PDF.
     - PyMuPDF (`fitz`) injects `toc.json` bookmark outlines and applies deflate compression.
