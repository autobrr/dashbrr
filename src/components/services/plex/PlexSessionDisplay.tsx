import React from "react";
import { PlexSession } from "../../../types/service";

interface PlexSessionDisplayProps {
  sessions: PlexSession[];
  isLoading?: boolean;
}

export const PlexSessionDisplay: React.FC<PlexSessionDisplayProps> = ({
  sessions,
  isLoading = false,
}) => {
  if (isLoading) {
    return (
      <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 animate-pulse">
        Loading sessions...
      </div>
    );
  }

  if (!sessions || sessions.length === 0) {
    return (
      <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
        No active sessions
      </div>
    );
  }

  return (
    <div className="mt-2 space-y-4">
      <div>
        <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
          Active Sessions ({sessions.length}):
        </div>
        <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2">
          {sessions.map((session, index) => (
            <div
              key={index}
              className="flex flex-col space-y-1 overflow-hidden"
            >
              <div className="text-xs opacity-75">
                <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                  Title:{" "}
                </span>
                <span className="text-xs overflow-hidden">
                  {session.type === "show" && session.grandparentTitle
                    ? `${session.grandparentTitle} - ${session.title}`
                    : session.type === "track" && session.grandparentTitle
                    ? `${session.grandparentTitle} - ${session.title}`
                    : session.type !== "show" &&
                      session.type !== "track" &&
                      session.grandparentTitle
                    ? session.grandparentTitle
                    : session.title}
                </span>
              </div>
              {session.User && (
                <div className="text-xs opacity-75">
                  <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                    User:{" "}
                  </span>
                  {session.User.title}
                </div>
              )}
              {session.Player && (
                <>
                  <div className="text-xs opacity-75">
                    <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                      IP:{" "}
                    </span>
                    {session.Player.remotePublicAddress}
                  </div>
                  <div className="text-xs opacity-75">
                    <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                      Device:{" "}
                    </span>
                    {session.Player.product} ({session.Player.device})
                  </div>
                </>
              )}
              {session.TranscodeSession && (
                <>
                  <div className="text-xs opacity-75">
                    <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                      Transcode:{" "}
                    </span>
                    {session.TranscodeSession.videoDecision}/
                    {session.TranscodeSession.audioDecision}
                  </div>
                  <div className="mt-1 w-full bg-gray-700 rounded-full h-1.5">
                    <div
                      className="bg-blue-500 h-1.5 rounded-full"
                      style={{
                        width: `${session.TranscodeSession.progress}%`,
                      }}
                    />
                  </div>
                </>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
