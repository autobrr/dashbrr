/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useMemo, useState } from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { ArrMessage } from "../common/ArrMessage";
import { OverseerrMediaRequest } from "../../../types/service";
import { OverseerrRequestModal } from "./OverseerrRequestModal";
import {
  CheckCircleIcon,
  XCircleIcon,
  ClockIcon
} from "@heroicons/react/24/outline";
import { api } from "../../../utils/api";
import { toast } from "react-hot-toast";
import Toast from "../../Toast";
import { FaFilm, FaTv, FaUser } from "react-icons/fa";
import { ArrowTopRightOnSquareIcon } from "@heroicons/react/24/outline";
import { combineServiceMessage } from "../../../utils/serviceMessage";
import { CollapsibleSection } from "../../ui/CollapsibleSection";
import { useCollapsiblePreference } from "../../../hooks/useCollapsiblePreference";
import { serviceSectionCollapseKey } from "../../../utils/collapsePreferences";
import { useTranslation } from "react-i18next";
import {
  getRequestStatus,
  OVERSEERR_REQUEST_STATUS,
  resolveRequestStatus
} from "./status";

interface OverseerrStatsProps {
  instanceId: string;
}

const EMPTY_REQUESTS: OverseerrMediaRequest[] = [];

export const OverseerrStats: React.FC<OverseerrStatsProps> = ({
  instanceId,
}) => {
  const { t } = useTranslation();
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const serviceRequests = service?.stats?.overseerr?.requests ?? EMPTY_REQUESTS;
  const [statusOverrides, setStatusOverrides] = useState<Record<number, number>>(
    {}
  );

  const requests = useMemo(() => {
    if (Object.keys(statusOverrides).length === 0) return serviceRequests;
    return serviceRequests.map((req) => {
      const next = statusOverrides[req.id];
      return next ? { ...req, status: next } : req;
    });
  }, [serviceRequests, statusOverrides]);
  const pendingRequests = useMemo(
    () =>
      requests.filter(
        (req) =>
          getRequestStatus(req.status) === OVERSEERR_REQUEST_STATUS.PENDING
      ),
    [requests]
  );
  const recentNonPendingRequests = useMemo(
    () =>
      requests
        .filter(
          (req) =>
            getRequestStatus(req.status) !== OVERSEERR_REQUEST_STATUS.PENDING
        )
        .sort(
          (a, b) =>
            new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
        ),
    [requests]
  );
  const pendingCount = pendingRequests.length;
  const isLoading = !service || service.status === "loading";
  const error = service?.status === "error" ? service.message : null;

  const [selectedRequest, setSelectedRequest] =
    useState<OverseerrMediaRequest | null>(null);
  const [modalAction, setModalAction] = useState<"approve" | "reject" | null>(
    null
  );
  const { isExpanded, toggle } = useCollapsiblePreference(
    serviceSectionCollapseKey(instanceId, "overseerr:recent_requests"),
    true
  );

  const handleAction = async (
    request: OverseerrMediaRequest,
    action: "approve" | "reject"
  ) => {
    setSelectedRequest(request);
    setModalAction(action);
  };

  const handleConfirmAction = async () => {
    if (!selectedRequest || !modalAction) return;

    try {
      const status = modalAction === "approve" ? 2 : 3; // 2 for approved, 3 for declined
      await api.post(
        `/api/services/${instanceId}/overseerr/request/${selectedRequest.id}/${status}`
      );

      // Optimistic UI: override status locally; SSE refresh will reconcile.
      setStatusOverrides((prev) => ({ ...prev, [selectedRequest.id]: status }));

      // Show success toast
      toast.custom((t_toast) => (
        <Toast
          type="success"
          body={t("overseerr.successfully", { action: modalAction, title: selectedRequest.media.title || "media", defaultValue: `Successfully ${modalAction}d request for ${selectedRequest.media.title || "media"}` })}
          t={t_toast}
        />
      ));

      setSelectedRequest(null);
      setModalAction(null);
    } catch (error) {
      console.error("Failed to update request status:", error);
      toast.custom((t_toast) => (
        <Toast
          type="error"
          body={t("overseerr.failed_to", { action: modalAction, error: String(error), defaultValue: `Failed to ${modalAction} request: ${error}` })}
          t={t_toast}
        />
      ));
    }
  };

  if (isLoading) {
    return <p className="text-xs text-gray-500">{t("overseerr.loading_requests", "Loading requests...")}</p>;
  }

  if (error) {
    return <p className="text-xs text-gray-500">{t("status.error", "Error")}: {error}</p>;
  }

  // Combine service message with health message if available
  const message = combineServiceMessage(service);

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
    });
  };

  const getUserDisplayName = (
    requestedBy: OverseerrMediaRequest["requestedBy"]
  ) => {
    if (!requestedBy) return t("overseerr.unknown_user", "Unknown User");
    return (
      requestedBy.username ||
      requestedBy.plexUsername ||
      requestedBy.email ||
      t("overseerr.unknown_user", "Unknown User")
    );
  };

  const getMediaType = (request: OverseerrMediaRequest) => {
    return request.media.tvdbId ? t("overseerr.show", "Show") : t("overseerr.movie", "Movie");
  };

  const getMediaTitle = (request: OverseerrMediaRequest) => {
    if (request.media.title) {
      return request.media.title;
    }
    return request.media.tvdbId
      ? t("overseerr.tv_show_id", { id: request.media.tvdbId, defaultValue: `TV Show (TVDB: ${request.media.tvdbId})` })
      : t("overseerr.movie_id", { id: request.media.tmdbId, defaultValue: `Movie (TMDB: ${request.media.tmdbId})` });
  };

  const RequestItem = ({
    request,
    isPending,
  }: {
    request: OverseerrMediaRequest;
    isPending?: boolean;
  }) => {
    const status = resolveRequestStatus(request);
    const statusMeta = status.meta;
    const isPendingRequest =
      status.requestStatus === OVERSEERR_REQUEST_STATUS.PENDING;

    return (
      <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-3.5 hover:bg-gray-850/80 transition-colors">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            {statusMeta.tone === "pending" ? (
              <span className="text-yellow-500">
                <ClockIcon className="h-4 w-4" />
              </span>
            ) : statusMeta.tone === "success" ? (
              <span className="text-green-500">
                <CheckCircleIcon className="h-4 w-4" />
              </span>
            ) : statusMeta.tone === "error" ? (
              <span className="text-red-500">
                <XCircleIcon className="h-4 w-4" />
              </span>
            ) : (
              <span className="text-zinc-400">
                <ClockIcon className="h-4 w-4" />
              </span>
            )}
            {isPending ? (
              <div className="flex items-center justify-between flex-1">
                <span
                  className="text-xs font-medium text-gray-200 truncate"
                  title={getMediaTitle(request)}
                >
                  {getMediaTitle(request)}
                </span>
                <div className="flex items-center gap-2 ml-4 flex-shrink-0">
                  {request.media.tmdbId !== 0 && (
                    <a
                      href={`https://www.themoviedb.org/${
                        request.media.mediaType === "tv" ? "tv" : "movie"
                      }/${request.media.tmdbId}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-[10px] px-2 py-0.5 rounded-md bg-gray-800/50 border border-gray-700/50 text-gray-300 hover:text-blue-400"
                    >
                      TMDB
                      <ArrowTopRightOnSquareIcon className="h-3 w-3" />
                    </a>
                  )}
                  {request.media.tvdbId !== 0 && (
                    <a
                      href={`https://www.thetvdb.com/dereferrer/series/${request.media.tvdbId}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-[10px] px-2 py-0.5 rounded-md bg-gray-800/50 border border-gray-700/50 text-gray-300 hover:text-blue-400"
                    >
                      TVDB
                      <ArrowTopRightOnSquareIcon className="h-3 w-3" />
                    </a>
                  )}
                </div>
              </div>
            ) : (
              <a
                href={request.media.serviceUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs font-medium text-gray-200 truncate hover:text-blue-400 transition-colors flex items-center"
                title="View Details"
              >
                {getMediaTitle(request)}
                <ArrowTopRightOnSquareIcon className="h-3.5 w-3.5 ml-1 text-blue-400" />
              </a>
            )}
          </div>
          <div className="flex items-center justify-between">
            <div className="flex flex-wrap items-center gap-y-1.5 text-xs text-gray-400">
              <span className="flex items-center gap-2 bg-gray-800/50 -ml-1 px-1.5 py-0.5 rounded">
                {getMediaType(request) === "Show" ? (
                  <FaTv className="h-3.5 w-3.5 text-gray-400" />
                ) : (
                  <FaFilm className="h-3.5 w-3.5 text-gray-400" />
                )}
                {getMediaType(request)}
              </span>
              <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                <FaUser className="h-3.5 w-3.5 text-gray-400" />
                {getUserDisplayName(request.requestedBy)}
              </span>
              <span className="flex items-center gap-1.5 bg-gray-800/50 px-2 py-0.5 rounded">
                <ClockIcon className="h-3.5 w-3.5 text-gray-400" />
                {formatDate(request.createdAt)}
              </span>
            </div>
            <div className="flex items-center gap-2 flex-shrink-0">
              {isPendingRequest ? (
                <>
                  <button
                    onClick={() => handleAction(request, "approve")}
                    className="text-green-500 hover:text-green-400 p-1.5 hover:bg-gray-700/50 rounded transition-colors"
                    title="Approve request"
                  >
                    <CheckCircleIcon className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => handleAction(request, "reject")}
                    className="text-red-500 hover:text-red-400 p-1.5 hover:bg-gray-700/50 rounded transition-colors"
                    title="Reject request"
                  >
                    <XCircleIcon className="h-4 w-4" />
                  </button>
                </>
              ) : (
                <span
                  className={`${statusMeta.color} bg-gray-800/50 px-2 py-0.5 rounded font-medium`}
                >
                  {t(`overseerr.status.${statusMeta.label.toLowerCase().replace(" ", "_")}`, statusMeta.label)}
                  {status.isFallback ? t("overseerr.media", " (media)") : ""}
                </span>
              )}
            </div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-4">
      <ArrMessage status={service.status} message={message} />

      {/* Pending Requests */}
      {pendingCount > 0 && (
        <div>
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300 cursor-default">
            {t("overseerr.pending_requests", "Pending Requests:")}
          </div>
          <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
            {pendingRequests
              .sort(
                (a, b) =>
                  new Date(b.createdAt).getTime() -
                  new Date(a.createdAt).getTime()
              )
              .slice(0, 3)
              .map((request) => (
                <RequestItem
                  key={request.id}
                  request={request}
                  isPending={true}
                />
              ))}
          </div>
          {pendingCount > 3 && (
            <div className="mt-1 text-[11px] text-zinc-400">
              {t("overseerr.showing_requests", { count: 3, total: pendingCount, defaultValue: `Showing 3 of ${pendingCount} pending requests` })}
            </div>
          )}
        </div>
      )}

      {/* Recent Requests */}
      {recentNonPendingRequests.length > 0 ? (
        <CollapsibleSection
          title={t("overseerr.recent_requests", "Recent Requests")}
          meta={`${Math.min(recentNonPendingRequests.length, 5)} shown`}
          isExpanded={isExpanded}
          onToggle={toggle}
        >
          <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
            {recentNonPendingRequests
              .slice(0, 5)
              .map((request) => (
                <RequestItem key={request.id} request={request} />
              ))}
          </div>
        </CollapsibleSection>
      ) : (
        <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
          {t("overseerr.no_recent", "No recent requests")}
        </div>
      )}

      {/* Confirmation Modal */}
      {selectedRequest && modalAction && (
        <OverseerrRequestModal
          isOpen={true}
          onClose={() => {
            setSelectedRequest(null);
            setModalAction(null);
          }}
          request={selectedRequest}
          onConfirm={handleConfirmAction}
          action={modalAction}
        />
      )}
    </div>
  );
};
