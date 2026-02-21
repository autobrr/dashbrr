import React from "react";
import { createRoot } from "react-dom/client";

import { CollapsibleSection } from "../../src/components/ui/CollapsibleSection";
import { UIPreferencesContext } from "../../src/contexts/UIPreferencesContext";
import { useCollapsiblePreference } from "../../src/hooks/useCollapsiblePreference";
import { serviceSectionCollapseKey } from "../../src/utils/collapsePreferences";
import { getServiceCardLayoutClasses } from "../../src/utils/serviceCardContent";

const compact = getServiceCardLayoutClasses("compact");
const regular = getServiceCardLayoutClasses("regular");

const app = document.getElementById("app");

if (!app) {
  throw new Error("missing app mount");
}

const PreferenceSection = ({
  testId,
  title,
  preferenceKey,
}: {
  testId: string;
  title: string;
  preferenceKey: string;
}) => {
  const { isExpanded, toggle } = useCollapsiblePreference(preferenceKey, true);

  return React.createElement(
    "div",
    { "data-testid": testId },
    React.createElement(
      CollapsibleSection,
      {
        title,
        isExpanded,
        onToggle: toggle,
      },
      React.createElement(
        "div",
        {
          className: "h-8 rounded bg-zinc-900",
        },
        title
      )
    )
  );
};

const PreferenceHarness = () => {
  const [collapsePreferences, setCollapsePreferences] = React.useState<
    Record<string, boolean>
  >({});
  const [renderKey, setRenderKey] = React.useState(0);

  const contextValue = React.useMemo(
    () => ({
      getCollapsed: (key: string, defaultCollapsed = false) =>
        collapsePreferences[key] ?? defaultCollapsed,
      setCollapsed: async (key: string, collapsed: boolean) => {
        setCollapsePreferences((current) => ({ ...current, [key]: collapsed }));
      },
    }),
    [collapsePreferences]
  );

  return React.createElement(
    UIPreferencesContext.Provider,
    { value: contextValue },
    React.createElement(
      "div",
      {
        className: "rounded-lg border border-zinc-700 bg-zinc-800 p-3 space-y-3",
      },
      React.createElement(
        "button",
        {
          "data-testid": "prefs-remount",
          onClick: () => setRenderKey((value) => value + 1),
          className:
            "rounded bg-zinc-700 px-2 py-1 text-[11px] font-medium text-zinc-200",
        },
        "remount"
      ),
      React.createElement(
        "div",
        { key: renderKey, className: "space-y-3" },
        React.createElement(PreferenceSection, {
          testId: "prefs-qui",
          title: "Active qBittorrent Instances",
          preferenceKey: serviceSectionCollapseKey("qui-1", "qui:active_instances"),
        }),
        React.createElement(PreferenceSection, {
          testId: "prefs-radarr",
          title: "Queue",
          preferenceKey: serviceSectionCollapseKey("radarr-1", "radarr:queue"),
        })
      )
    )
  );
};

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
    ),
    React.createElement(
      "div",
      {
        "data-testid": "prefs-card",
      },
      React.createElement(PreferenceHarness)
    )
  );
};

createRoot(app).render(React.createElement(Harness));
