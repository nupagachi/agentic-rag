import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Sheet, SheetContent, SheetDescription, SheetTitle } from "./sheet";

describe("SheetContent close button", () => {
  it("renders no close button by default", () => {
    render(
      <Sheet open>
        <SheetContent>
          <SheetTitle>Title</SheetTitle>
          <SheetDescription>Body</SheetDescription>
        </SheetContent>
      </Sheet>,
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders a close button named by closeLabel when enabled", () => {
    render(
      <Sheet open>
        <SheetContent showCloseButton closeLabel="Dong">
          <SheetTitle>Title</SheetTitle>
          <SheetDescription>Body</SheetDescription>
        </SheetContent>
      </Sheet>,
    );
    expect(screen.getByRole("button", { name: "Dong" })).toBeInTheDocument();
    expect(screen.queryByText("Close")).not.toBeInTheDocument();
  });

  it("rejects invalid prop combinations at compile time", () => {
    // @ts-expect-error showCloseButton requires closeLabel
    const missingLabel = <SheetContent showCloseButton />;
    // @ts-expect-error closeLabel is not allowed without showCloseButton
    const strayLabel = <SheetContent closeLabel="x" />;
    expect(missingLabel).toBeTruthy();
    expect(strayLabel).toBeTruthy();
  });
});
