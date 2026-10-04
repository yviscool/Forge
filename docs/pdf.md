# PDF export workflow

`GET /api/contests/{contestID}/problems/{problemID}/export?locale=zh-CN` returns a self-contained, print-ready HTML document. In Chromium/Edge use **Print -> Save as PDF**, enable background graphics, and use A4 margins. For automation, Playwright can call `page.goto(url)` and `page.pdf({ format: 'A4', printBackground: true })`. The supplied CSP PDFs remain visual references; exact typography and page-break rules are intentionally a follow-up layout task.
