import { createContext } from "react";
import { ConfigurationContextType } from "./types";

export const ConfigurationContext = createContext<ConfigurationContextType | undefined>(undefined);
