/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useEffect, useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "../../contexts/AuthContext";
import { RegisterCredentials } from "../../types/auth";
import { toast } from "react-hot-toast";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { faOpenid } from "@fortawesome/free-brands-svg-icons";
import { faCheck, faTimes } from "@fortawesome/free-solid-svg-icons";
import Toast from "../Toast";
import logo from "../../assets/logo.svg";
import { Footer } from "../shared/Footer";

export function LoginPage() {
  const {
    isAuthenticated,
    loading,
    login,
    loginWithOIDC,
    register,
    authConfig,
  } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [isRegistering, setIsRegistering] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [registrationEnabled, setRegistrationEnabled] =
    useState<boolean>(false);
  const [checkingRegistration, setCheckingRegistration] = useState(true);

  // Form state
  const [formData, setFormData] = useState<
    RegisterCredentials & { confirmPassword?: string }
  >({
    username: "",
    password: "",
    confirmPassword: "",
    email: "", // Will be set during registration
  });

  // Password validation state
  const [passwordValidation, setPasswordValidation] = useState({
    minLength: false,
    hasUppercase: false,
    hasLowercase: false,
    hasNumber: false,
    passwordsMatch: false,
  });

  // Get the return URL from location state, or default to '/'
  const from =
    (location.state as { from?: { pathname: string } })?.from?.pathname || "/";

  useEffect(() => {
    // Only check registration status if built-in auth is enabled
    const checkRegistrationStatus = async () => {
      if (!authConfig?.methods.builtin) {
        setCheckingRegistration(false);
        return;
      }

      try {
        const response = await fetch("/api/auth/registration-status");
        const data = await response.json();
        setRegistrationEnabled(data.registrationEnabled);
        if (data.registrationEnabled && !data.hasUsers) {
          // Automatically switch to registration if no users exist
          setIsRegistering(true);
        }
      } catch (err) {
        console.error("Failed to check registration status:", err);
        setRegistrationEnabled(false);
      } finally {
        setCheckingRegistration(false);
      }
    };

    if (authConfig) {
      checkRegistrationStatus();
    }
  }, [authConfig]);

  useEffect(() => {
    // If already authenticated, redirect to the return URL
    if (isAuthenticated && !loading) {
      navigate(from, { replace: true });
    }
  }, [isAuthenticated, loading, navigate, from]);

  // Password validation effect
  useEffect(() => {
    if (isRegistering) {
      const password = formData.password;
      setPasswordValidation({
        minLength: password.length >= 8,
        hasUppercase: /[A-Z]/.test(password),
        hasLowercase: /[a-z]/.test(password),
        hasNumber: /[0-9]/.test(password),
        passwordsMatch:
          password === formData.confirmPassword && password !== "",
      });
    }
  }, [formData.password, formData.confirmPassword, isRegistering]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]: value,
    }));
    setError(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    try {
      if (isRegistering) {
        // Check if passwords match
        if (formData.password !== formData.confirmPassword) {
          setError("Passwords do not match");
          toast.custom((t) => (
            <Toast type="error" body="Passwords do not match" t={t} />
          ));
          return;
        }

        // Check if all password requirements are met
        const allRequirementsMet = Object.values(passwordValidation).every(
          (value) => value
        );
        if (!allRequirementsMet) {
          setError("Please meet all password requirements");
          toast.custom((t) => (
            <Toast
              type="error"
              body="Please meet all password requirements"
              t={t}
            />
          ));
          return;
        }

        try {
          // Generate a default email using the username
          const defaultEmail = `${formData.username}@dashbrr.local`;

          await register({
            username: formData.username,
            password: formData.password,
            email: defaultEmail,
          });
          toast.custom((t) => (
            <Toast type="success" body="Registration successful!" t={t} />
          ));
          setIsRegistering(false); // Switch back to login view
        } catch (err) {
          const errorMessage = err instanceof Error ? err.message : String(err);
          // Check for registration disabled error
          if (errorMessage.includes("Registration is disabled")) {
            toast.custom((t) => (
              <Toast
                type="error"
                body="Registration is disabled. A user already exists."
                t={t}
              />
            ));
            setIsRegistering(false); // Switch back to login mode
          } else {
            toast.custom((t) => (
              <Toast type="error" body={errorMessage} t={t} />
            ));
          }
          setError(errorMessage);
          return;
        }
      } else {
        try {
          await login({
            username: formData.username,
            password: formData.password,
          });
          toast.custom((t) => (
            <Toast type="success" body="Login successful!" t={t} />
          ));
        } catch (err) {
          const errorMessage = err instanceof Error ? err.message : String(err);
          // Check if the error indicates no users exist
          if (errorMessage.includes("User not found") && registrationEnabled) {
            setIsRegistering(true); // Switch to registration mode
            toast.custom((t) => (
              <Toast
                type="info"
                body="No user found. Please register a new account."
                t={t}
              />
            ));
          } else {
            setError(errorMessage);
            toast.custom((t) => (
              <Toast type="error" body={errorMessage} t={t} />
            ));
          }
        }
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Authentication failed";
      setError(errorMessage);
      toast.custom((t) => <Toast type="error" body={errorMessage} t={t} />);
    }
  };

  if (
    loading ||
    !authConfig ||
    (authConfig.methods.builtin && checkingRegistration)
  ) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-900 pattern">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-900 pattern">
      <div className="max-w-md w-full space-y-8 p-8 bg-gray-850/40 border border-black/40 rounded-lg shadow-lg">
        <div className="flex flex-col items-center">
          <img
            src={logo}
            alt="Dashbrr Logo"
            className="text-white h-16 w-16 mb-2 select-none pointer-events-none"
          />
          <h2 className="text-3xl font-bold text-white pointer-events-none select-none">
            Dashbrr
          </h2>
          <p className="mt-2 text-sm text-gray-400">
            {authConfig.methods.builtin
              ? isRegistering
                ? "Create your account"
                : "Sign in to your account"
              : ""}
          </p>
        </div>

        {error && (
          <div className="bg-red-500 bg-opacity-10 border border-red-500 text-red-500 px-4 py-3 rounded">
            <span className="block sm:inline">{error}</span>
          </div>
        )}

        {authConfig.methods.builtin && (
          <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
            <div className="rounded-md shadow-sm -space-y-px">
              <div>
                <label htmlFor="username" className="sr-only">
                  Username
                </label>
                <input
                  id="username"
                  name="username"
                  type="text"
                  autoComplete="username"
                  required
                  className="appearance-none rounded-t-md relative block w-full px-3 py-2 border border-gray-700 dark:border-gray-900 bg-gray-700 text-gray-300 placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm"
                  placeholder="Username"
                  value={formData.username}
                  onChange={handleInputChange}
                />
              </div>
              <div>
                <label htmlFor="password" className="sr-only">
                  Password
                </label>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  className={`appearance-none relative block w-full px-3 py-2 border border-gray-700 dark:border-gray-900 bg-gray-700 text-gray-300 placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm ${
                    !isRegistering ? "rounded-b-md" : ""
                  }`}
                  placeholder="Password"
                  value={formData.password}
                  onChange={handleInputChange}
                />
              </div>
              {isRegistering && (
                <div>
                  <label htmlFor="confirmPassword" className="sr-only">
                    Confirm Password
                  </label>
                  <input
                    id="confirmPassword"
                    name="confirmPassword"
                    type="password"
                    required
                    className="appearance-none rounded-b-md relative block w-full px-3 py-2 border border-gray-700 dark:border-gray-900 bg-gray-700 text-gray-300 placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm"
                    placeholder="Confirm Password"
                    value={formData.confirmPassword}
                    onChange={handleInputChange}
                  />
                </div>
              )}
            </div>

            {isRegistering && (
              <div className="rounded-md bg-blue-900 bg-opacity-20 p-4">
                <div className="text-sm">
                  <h4 className="font-medium mb-2 text-blue-400">
                    Password Requirements:
                  </h4>
                  <ul className="space-y-1">
                    <li
                      className={`flex items-center ${
                        passwordValidation.minLength
                          ? "text-green-400"
                          : "text-blue-400"
                      }`}
                    >
                      <FontAwesomeIcon
                        icon={passwordValidation.minLength ? faCheck : faTimes}
                        className="w-4 h-4 mr-2"
                      />
                      Minimum 8 characters
                    </li>
                    <li
                      className={`flex items-center ${
                        passwordValidation.hasUppercase
                          ? "text-green-400"
                          : "text-blue-400"
                      }`}
                    >
                      <FontAwesomeIcon
                        icon={
                          passwordValidation.hasUppercase ? faCheck : faTimes
                        }
                        className="w-4 h-4 mr-2"
                      />
                      At least one uppercase letter
                    </li>
                    <li
                      className={`flex items-center ${
                        passwordValidation.hasLowercase
                          ? "text-green-400"
                          : "text-blue-400"
                      }`}
                    >
                      <FontAwesomeIcon
                        icon={
                          passwordValidation.hasLowercase ? faCheck : faTimes
                        }
                        className="w-4 h-4 mr-2"
                      />
                      At least one lowercase letter
                    </li>
                    <li
                      className={`flex items-center ${
                        passwordValidation.hasNumber
                          ? "text-green-400"
                          : "text-blue-400"
                      }`}
                    >
                      <FontAwesomeIcon
                        icon={passwordValidation.hasNumber ? faCheck : faTimes}
                        className="w-4 h-4 mr-2"
                      />
                      At least one number
                    </li>
                    <li
                      className={`flex items-center ${
                        passwordValidation.passwordsMatch
                          ? "text-green-400"
                          : "text-blue-400"
                      }`}
                    >
                      <FontAwesomeIcon
                        icon={
                          passwordValidation.passwordsMatch ? faCheck : faTimes
                        }
                        className="w-4 h-4 mr-2"
                      />
                      Passwords match
                    </li>
                  </ul>
                </div>
              </div>
            )}

            <div>
              <button
                type="submit"
                className="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              >
                {isRegistering ? "Register" : "Sign in"}
              </button>
            </div>
          </form>
        )}

        {authConfig.methods.oidc && (
          <div className={authConfig.methods.builtin ? "mt-4" : "mt-8"}>
            {authConfig.methods.builtin && (
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-gray-700"></div>
                </div>
                <div className="relative flex justify-center text-sm">
                  <span className="px-2 bg-gray-800 text-gray-400">
                    Or continue with
                  </span>
                </div>
              </div>
            )}

            <div className={authConfig.methods.builtin ? "mt-6 " : ""}>
              <button
                onClick={() => loginWithOIDC()}
                className="w-full flex justify-center items-center py-2 px-4 border border-gray-750 rounded-md shadow-sm bg-gray-800 hover:bg-gray-825 text-sm font-medium text-white hover:text-blue-450 focus:outline-none focus:ring-1  focus:ring-gray-700"
              >
                <span
                  className="group relative inline-block"
                  aria-label="Sign in with OpenID"
                >
                  Sign in with
                  <FontAwesomeIcon
                    icon={faOpenid}
                    className="text-lg ml-2"
                    aria-hidden="true"
                  />
                </span>
              </button>
            </div>
          </div>
        )}
        <Footer />
      </div>
    </div>
  );
}
