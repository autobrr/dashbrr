/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ServiceCard } from "./ServiceCard";
import { Service } from "../../types/service";
import LoadingSkeleton from "../shared/LoadingSkeleton";
import { useState, useEffect, useRef, CSSProperties } from "react";
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  TouchSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragEndEvent,
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  horizontalListSortingStrategy,
  useSortable,
} from "@dnd-kit/sortable";

interface ServiceGridProps {
  services: Service[];
  onRemoveService: (instanceId: string) => void;
  isConnected?: boolean;
  isLoading?: boolean;
}

// Wrapper component to make ServiceCard draggable
const DraggableServiceCard = ({
  service,
  onRemove,
  isConnected,
  isInitialLoad,
}: {
  service: Service;
  onRemove: () => void;
  isConnected: boolean;
  isInitialLoad?: boolean;
}) => {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: service.instanceId });

  const style: CSSProperties | undefined = transform
    ? {
        transform: `translate3d(${transform.x}px, ${transform.y}px, 0)`,
        transition,
        zIndex: isDragging ? 2 : undefined,
        position: isDragging ? ("relative" as const) : undefined,
        opacity: isDragging ? 0.8 : undefined,
      }
    : undefined;

  return (
    <div ref={setNodeRef} style={style} className="mb-4 break-inside-avoid">
      <ServiceCard
        service={service}
        onRemove={onRemove}
        isConnected={isConnected}
        isInitialLoad={isInitialLoad}
        dragHandleProps={{ ...attributes, ...listeners }}
        isDragging={isDragging}
      />
    </div>
  );
};

export const ServiceGrid = ({
  services = [],
  onRemoveService,
  isConnected = true,
  isLoading = false,
}: ServiceGridProps) => {
  const [items, setItems] = useState<Service[]>([]);
  const prevServicesRef = useRef<Service[]>([]);

  // Sort services based on saved order
  const sortServices = (services: Service[]) => {
    try {
      const savedOrder = window.localStorage.getItem("dashbrr-service-order");
      if (!savedOrder) return services;

      const orderMap = new Map<string, number>(JSON.parse(savedOrder));
      return [...services].sort((a, b) => {
        const orderA = orderMap.get(a.instanceId) ?? Number.MAX_SAFE_INTEGER;
        const orderB = orderMap.get(b.instanceId) ?? Number.MAX_SAFE_INTEGER;
        return orderA - orderB;
      });
    } catch (error) {
      console.error("Error sorting services:", error);
      return services;
    }
  };

  // Helper function to check if two services have different content
  const hasServiceContentChanged = (a: Service, b: Service): boolean => {
    return (
      a.url !== b.url ||
      a.accessUrl !== b.accessUrl ||
      a.displayName !== b.displayName ||
      a.version !== b.version ||
      a.updateAvailable !== b.updateAvailable ||
      a.status !== b.status
    );
  };

  // Initialize and update items
  useEffect(() => {
    if (services.length === 0) {
      setItems([]);
      return;
    }

    // Check if services have changed (either order or content)
    const servicesChanged = services.some((service, index) => {
      const prevService = prevServicesRef.current[index];
      return (
        !prevService ||
        prevService.instanceId !== service.instanceId ||
        hasServiceContentChanged(service, prevService)
      );
    });

    if (!servicesChanged && items.length > 0) {
      // Update service data while maintaining current order
      setItems((prev) =>
        prev.map((item) => {
          const updatedService = services.find(
            (s) => s.instanceId === item.instanceId
          );
          return updatedService || item;
        })
      );
    } else {
      // Sort and set new services
      const sortedServices = sortServices(services);
      setItems(sortedServices);
    }

    prevServicesRef.current = services;
  }, [services, items.length]);

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;

    if (over && active.id !== over.id) {
      setItems((items) => {
        const oldIndex = items.findIndex(
          (item) => item.instanceId === active.id
        );
        const newIndex = items.findIndex((item) => item.instanceId === over.id);
        const newItems = arrayMove(items, oldIndex, newIndex);

        try {
          // Save the new order to localStorage
          const orderMap = new Map(
            newItems.map((item, index) => [item.instanceId, index])
          );
          window.localStorage.setItem(
            "dashbrr-service-order",
            JSON.stringify([...orderMap])
          );
        } catch (error) {
          console.error("Error saving service order:", error);
        }

        return newItems;
      });
    }
  };

  const handleRemoveService = async (instanceId: string) => {
    // Immediately update local state
    setItems((prev) => prev.filter((item) => item.instanceId !== instanceId));

    // Call the parent's onRemoveService
    await onRemoveService(instanceId);
  };

  const isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);

  const touchSensor = useSensor(TouchSensor, {
    activationConstraint: {
      delay: 200,
      tolerance: 8,
    },
  });
  const pointerSensor = useSensor(PointerSensor);
  const keyboardSensor = useSensor(KeyboardSensor, {
    coordinateGetter: sortableKeyboardCoordinates,
  });

  const sensors = useSensors(
    ...(isMobile ? [touchSensor] : [pointerSensor]),
    keyboardSensor
  );

  if (isLoading) {
    return (
      <div className="grid grid-cols-[repeat(auto-fit,minmax(300px,1fr))] hover:cursor-pointer gap-6 px-0 py-6 animate-fadeIn">
        {[...Array(4)].map((_, i) => (
          <LoadingSkeleton key={i} />
        ))}
      </div>
    );
  }

  if (!services || services.length === 0) {
    return (
      <div className="flex items-center justify-center h-[calc(100vh-12rem)] w-full">
        <div className="text-center p-8 rounded-lg bg-gray-50 dark:bg-gray-800/50 backdrop-blur-sm">
          <h3 className="text-xl font-medium text-gray-900 dark:text-white mb-3">
            No Services Configured
          </h3>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Click the "Add Service" button to get started.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full mt-4">
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
      >
        <SortableContext
          items={items.map((item) => item.instanceId)}
          strategy={horizontalListSortingStrategy}
        >
          <div
            className="columns-1 sm:columns-1 md:columns-2 lg:columns-3 xl:columns-4 2xl:columns-5 3xl:columns-6 gap-6"
            style={{ columnFill: "balance", width: "100%" }}
          >
            {items.map((service) => (
              <DraggableServiceCard
                key={service.instanceId}
                service={service}
                onRemove={() => handleRemoveService(service.instanceId)}
                isConnected={isConnected}
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>
    </div>
  );
};
