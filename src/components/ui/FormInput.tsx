import React, { useState } from "react";
import { EyeIcon, EyeSlashIcon } from "@heroicons/react/24/solid";

interface FormInputProps {
  id: string;
  label: string;
  type?: "text" | "password";
  value: string;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  placeholder?: string;
  required?: boolean;
  helpText?: {
    prefix?: string;
    text: string;
    link?: string | null;
  };
}

export const FormInput: React.FC<FormInputProps> = ({
  id,
  label,
  type = "text",
  value,
  onChange,
  placeholder,
  required = false,
  helpText,
}) => {
  const [isVisible, setIsVisible] = useState(false);
  const isPassword = type === "password";

  return (
    <div className="mb-4">
      <label
        htmlFor={id}
        className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
      >
        {label}
      </label>
      <div className="relative">
        <input
          type={isPassword ? (isVisible ? "text" : "password") : type}
          id={id}
          value={value}
          onChange={onChange}
          className="w-full px-3 pr-10 py-2 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          placeholder={placeholder}
          required={required}
          data-1p-ignore={isPassword}
        />
        {isPassword && (
          <div
            className="absolute inset-y-0 right-0 px-3 flex items-center cursor-pointer"
            onClick={() => setIsVisible(!isVisible)}
          >
            {!isVisible ? (
              <EyeIcon
                className="h-5 w-5 text-gray-400 hover:text-gray-500"
                aria-hidden="true"
              />
            ) : (
              <EyeSlashIcon
                className="h-5 w-5 text-gray-400 hover:text-gray-500"
                aria-hidden="true"
              />
            )}
          </div>
        )}
      </div>
      {helpText && (
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {helpText.prefix}
          {helpText.link ? (
            <a
              href={helpText.link}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-500 hover:text-blue-600 dark:text-blue-400 dark:hover:text-blue-300"
            >
              {helpText.text}
            </a>
          ) : (
            helpText.text
          )}
        </p>
      )}
    </div>
  );
};
