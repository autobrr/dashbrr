import React, { ReactNode } from "react";

interface CardProps {
  children: ReactNode;
  className?: string;
  header?: ReactNode;
  footer?: ReactNode;
  onClick?: () => void;
  hoverable?: boolean;
  variant?: "default" | "primary" | "secondary";
  noPadding?: boolean;
}

export const Card: React.FC<CardProps> = ({
  children,
  className = "",
  header,
  footer,
  onClick,
  hoverable = false,
  variant = "default",
  noPadding = false,
}) => {
  const variants = {
    default: `
      bg-white dark:bg-gray-800 
      border border-gray-200 dark:border-gray-700
    `,
    primary: `
      bg-blue-50/50 dark:bg-blue-900/20
      border border-blue-100 dark:border-blue-800/50
    `,
    secondary: `
      bg-gray-50/50 dark:bg-gray-800/50
      border border-gray-200 dark:border-gray-700
    `,
  };

  const baseClasses = `
    relative
    rounded-lg
    transition-all duration-200
    overflow-hidden
  `;

  const hoverClasses = hoverable
    ? `
      cursor-pointer
      transform hover:scale-[1.01]
      active:scale-[0.99]
    `
    : "";

  const combinedClasses = `
    ${baseClasses}
    ${variants[variant]}
    ${hoverClasses}
    ${className}
  `
    .replace(/\s+/g, " ")
    .trim();

  return (
    <div className={combinedClasses} onClick={onClick}>
      {header && (
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          {header}
        </div>
      )}
      <div className={noPadding ? "" : "p-6"}>{children}</div>
      {footer && (
        <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-800/50">
          {footer}
        </div>
      )}
    </div>
  );
};
