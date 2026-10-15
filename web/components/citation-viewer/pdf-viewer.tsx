"use client";

import { useState } from "react";
import { Document, Page, pdfjs } from "react-pdf";
import "react-pdf/dist/Page/AnnotationLayer.css";
import "react-pdf/dist/Page/TextLayer.css";

pdfjs.GlobalWorkerOptions.workerSrc = new URL("pdfjs-dist/build/pdf.worker.min.mjs", import.meta.url).toString();

export interface PdfViewerProps {
  url: string;
  page?: number;
  /** Text to highlight on the page (citation span). */
  highlight?: string;
  width?: number;
}

function escapeHtml(s: string) {
  return s.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
}

/**
 * Load with `dynamic(() => import(...), { ssr: false })`: pdf.js needs
 * browser APIs and must not run during SSR.
 */
export default function PdfViewer({ url, page = 1, highlight, width }: PdfViewerProps) {
  const [numPages, setNumPages] = useState<number>();

  return (
    <Document file={url} onLoadSuccess={({ numPages }) => setNumPages(numPages)}>
      {numPages && (
        <Page
          pageNumber={Math.min(page, numPages)}
          width={width}
          customTextRenderer={({ str }) => {
            const safe = escapeHtml(str);
            if (!highlight || !str.includes(highlight)) return safe;
            return safe.replaceAll(escapeHtml(highlight), (m) => `<mark>${m}</mark>`);
          }}
        />
      )}
    </Document>
  );
}
