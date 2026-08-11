import { Menu, Transition } from "@headlessui/react";
import { Fragment } from "react";
import { GlobeAltIcon } from "@heroicons/react/20/solid";
import { useTranslation } from "react-i18next";
import clsx from "clsx";

const languages = [
  { code: "en", nameKey: "languages.en" },
  { code: "fr", nameKey: "languages.fr" },
];

export function LanguageSelector() {
  const { t, i18n } = useTranslation();

  return (
    <Menu as="div" className="relative inline-block text-left">
      <div>
        <Menu.Button
          className="flex items-center gap-1 p-1 text-zinc-400 hover:text-zinc-600 dark:hover:text-white rounded focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          title={t("common.language_selector", "Change Language")}
          aria-label={t("common.language_selector", "Change Language")}
        >
          <GlobeAltIcon className="h-5 w-5" aria-hidden="true" />
          <span className="text-xs font-medium uppercase">{i18n.resolvedLanguage || "en"}</span>
        </Menu.Button>
      </div>

      <Transition
        as={Fragment}
        enter="transition ease-out duration-100"
        enterFrom="transform opacity-0 scale-95"
        enterTo="transform opacity-100 scale-100"
        leave="transition ease-in duration-75"
        leaveFrom="transform opacity-100 scale-100"
        leaveTo="transform opacity-0 scale-95"
      >
        <Menu.Items className="absolute right-0 z-50 mt-2 w-36 origin-top-right rounded-md bg-white dark:bg-zinc-800 shadow-lg ring-1 ring-black/5 focus:outline-none">
          <div className="py-1">
            {languages.map((lng) => (
              <Menu.Item key={lng.code}>
                {({ active }) => (
                  <button
                    onClick={() => i18n.changeLanguage(lng.code)}
                    className={clsx(
                      active ? "bg-zinc-100 dark:bg-zinc-700 text-zinc-900 dark:text-white" : "text-zinc-700 dark:text-zinc-300",
                      i18n.resolvedLanguage === lng.code ? "font-bold" : "font-medium",
                      "block w-full px-4 py-2 text-left text-sm"
                    )}
                  >
                    {t(lng.nameKey)}
                  </button>
                )}
              </Menu.Item>
            ))}
          </div>
        </Menu.Items>
      </Transition>
    </Menu>
  );
}
