"use client";

import dynamic from "next/dynamic";

import { Skeleton } from "@/components/ui/skeleton";

export const PdfViewer = dynamic(() => import("./pdf-viewer"), {
  ssr: false,
  loading: () => <Skeleton className="h-[600px] w-full" />,
});
