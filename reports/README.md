# Reports

- **reportHI10.pdf** - Hand-in 10 (Exercise 16.2: Static Proof-of-Stake, throughput, rollback).

Source for Hand-in 10: `reportHI10.md`.

Generate the PDF from the repo root:

```bash
pandoc reports/reportHI10.md -o reports/reportHI10.pdf --pdf-engine=pdflatex
```
# Reports

- **reportHI1.pdf** – Hand-in 1 (Peer-to-Peer Flooding and Simple Ledger).
- **reportHI10.pdf** – Hand-in 10 (Exercise 16.2: Static Proof-of-Stake, throughput, rollback).
- **reportHI3.pdf** – Hand-in 3 (Validation and measurements).

Source for Hand-in 3: `report.md`.
Source for Hand-in 10: `reportHI10.md`.

To regenerate the Hand-in 3 PDF from the repo root:

```bash
pandoc reports/report.md -o reports/reportHI3.pdf --pdf-engine=pdflatex -H reports/report-header.tex -V colorlinks=true
```

To generate the Hand-in 10 PDF from the repo root:

```bash
pandoc reports/reportHI10.md -o reports/reportHI10.pdf --pdf-engine=pdflatex
```
