import React, { createContext, useState, useEffect } from "react";
import NotificationService from "../services/NotificationService";

interface NotificationContextType {
  notificationsEnabled: boolean;
  toggleNotifications: () => void;
  requestPermission: () => Promise<boolean>;
  notifyServiceUpdate: (
    serviceName: string,
    status: string,
    message?: string
  ) => Promise<boolean>;
  notifyVersionUpdate: (
    serviceName: string,
    currentVersion: string,
    newVersion: string
  ) => Promise<boolean>;
}

const NotificationContext = createContext<NotificationContextType | undefined>(
  undefined
);

export const NotificationProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [notificationsEnabled, setNotificationsEnabled] = useState<boolean>(
    () => {
      const stored = localStorage.getItem("notifications-enabled");
      return stored ? JSON.parse(stored) : true;
    }
  );

  useEffect(() => {
    localStorage.setItem(
      "notifications-enabled",
      JSON.stringify(notificationsEnabled)
    );
    NotificationService.setEnabled(notificationsEnabled);
  }, [notificationsEnabled]);

  const toggleNotifications = () => {
    setNotificationsEnabled((prev) => !prev);
  };

  const requestPermission = async () => {
    return NotificationService.requestPermission();
  };

  const notifyServiceUpdate = async (
    serviceName: string,
    status: string,
    message?: string
  ) => {
    return NotificationService.notifyServiceUpdate(
      serviceName,
      status,
      message
    );
  };

  const notifyVersionUpdate = async (
    serviceName: string,
    currentVersion: string,
    newVersion: string
  ) => {
    return NotificationService.notifyVersionUpdate(
      serviceName,
      currentVersion,
      newVersion
    );
  };

  return (
    <NotificationContext.Provider
      value={{
        notificationsEnabled,
        toggleNotifications,
        requestPermission,
        notifyServiceUpdate,
        notifyVersionUpdate,
      }}
    >
      {children}
    </NotificationContext.Provider>
  );
};

export { NotificationContext };
