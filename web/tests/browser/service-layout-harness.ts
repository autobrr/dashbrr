import React from "react";
import { createRoot } from "react-dom/client";

import { CollapsibleSection } from "../../src/components/ui/CollapsibleSection";
import { getServiceCardLayoutClasses } from "../../src/utils/serviceCardContent";

const compact = getServiceCardLayoutClasses("compact");
const regular = getServiceCardLayoutClasses("regular");

const app = document.getElementById("app");

if (!app) {
  throw new Error("missing app mount");
}

const Harness = () => {
  const [isExpanded, setIsExpanded] = React.useState(false);

  return React.createElement(
    "div",
    { className: "mx-auto w-full max-w-5xl space-y-4" },
    React.createElement(
      "div",
      { className: "grid grid-cols-1 gap-4 md:grid-cols-2" },
      React.createElement(
        "div",
        {
          "data-testid": "compact-card",
          className: "rounded-lg border border-zinc-700 bg-zinc-800 p-3",
        },
        React.createElement("div", { className: compact.bodyMarginClass }, "Compact body"),
        React.createElement(
          "div",
          {
            "data-testid": "compact-footer",
            className: compact.footerSpacingClass,
          },
          "Compact footer"
        )
      ),
      React.createElement(
        "div",
        {
          "data-testid": "regular-card",
          className: "rounded-lg border border-zinc-700 bg-zinc-800 p-3",
        },
        React.createElement("div", { className: regular.bodyMarginClass }, "Regular body"),
        React.createElement(
          "div",
          {
            "data-testid": "regular-footer",
            className: regular.footerSpacingClass,
          },
          "Regular footer"
        )
      )
    ),
    React.createElement(
      "div",
      {
        "data-testid": "shell-card",
        className: "rounded-lg border border-zinc-700 bg-zinc-800 p-3",
      },
      React.createElement(
        CollapsibleSection,
        {
          title: "Recent Releases",
          isExpanded,
          onToggle: () => setIsExpanded((value) => !value),
        },
        React.createElement(
          "div",
          {
            "data-testid": "shell-content",
            className: "h-10 rounded bg-zinc-900",
          },
          "Content"
        )
      )
    )
  );
};

createRoot(app).render(React.createElement(Harness));
