/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ServiceCard } from "./ServiceCard";
import { Service } from "../../types/service";
import LoadingSkeleton from "../shared/LoadingSkeleton";
import { useState, useEffect, CSSProperties } from "react";
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  TouchSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragEndEvent
} from "@dnd-kit/core";
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  rectSortingStrategy,
  useSortable
} from "@dnd-kit/sortable";

interface ServiceGridProps {
  services: Service[];
  onRemoveService: (instanceId: string) => void;
  isConnected?: boolean;
  isLoading?: boolean;
}

const SERVICE_ORDER_STORAGE_KEY = "dashbrr-service-order";

const readSavedOrder = (): Map<string, number> => {
  try {
    const raw = window.localStorage.getItem(SERVICE_ORDER_STORAGE_KEY);
    if (!raw) {
      return new Map();
    }
    const entries = JSON.parse(raw) as Array<[string, number]>;
    return new Map(entries);
  } catch (error) {
    console.error("Error reading service order:", error);
    return new Map();
  }
};

const sortBySavedOrder = (
  services: Service[],
  savedOrder: Map<string, number>
): Service[] => {
  return [...services].sort((a, b) => {
    const orderA = savedOrder.get(a.instanceId) ?? Number.MAX_SAFE_INTEGER;
    const orderB = savedOrder.get(b.instanceId) ?? Number.MAX_SAFE_INTEGER;
    return orderA - orderB;
  });
};

const persistOrder = (services: Service[]) => {
  try {
    const orderMap = new Map(
      services.map((service, index) => [service.instanceId, index])
    );
    window.localStorage.setItem(
      SERVICE_ORDER_STORAGE_KEY,
      JSON.stringify([...orderMap])
    );
  } catch (error) {
    console.error("Error saving service order:", error);
  }
};

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
    <div ref={setNodeRef} style={style} className="mb-4 break-inside-avoid sm:mb-6">
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
  const masonryClasses =
    "columns-1 gap-4 sm:gap-6 md:columns-2 2xl:columns-3 [@media(min-width:2200px)]:columns-4";

  // Initialize and update items
  useEffect(() => {
    setItems((prev) => {
      if (services.length === 0) {
        return [];
      }

      const byInstanceID = new Map(
        services.map((service) => [service.instanceId, service])
      );
      const savedOrder = readSavedOrder();

      if (prev.length === 0) {
        return sortBySavedOrder(services, savedOrder);
      }

      const existing = prev
        .map((service) => byInstanceID.get(service.instanceId))
        .filter((service): service is Service => Boolean(service));

      const existingIDs = new Set(existing.map((service) => service.instanceId));
      const additions = sortBySavedOrder(
        services.filter((service) => !existingIDs.has(service.instanceId)),
        savedOrder
      );

      return [...existing, ...additions];
    });
  }, [services]);

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;

    if (over && active.id !== over.id) {
      setItems((items) => {
        const oldIndex = items.findIndex(
          (item) => item.instanceId === active.id
        );
        const newIndex = items.findIndex((item) => item.instanceId === over.id);
        const newItems = arrayMove(items, oldIndex, newIndex);
        persistOrder(newItems);
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

  const touchSensor = useSensor(TouchSensor, {
    activationConstraint: {
      delay: 180,
      tolerance: 6,
    },
  });
  const pointerSensor = useSensor(PointerSensor, {
    activationConstraint: {
      distance: 8,
    },
  });
  const keyboardSensor = useSensor(KeyboardSensor, {
    coordinateGetter: sortableKeyboardCoordinates,
  });

  const sensors = useSensors(pointerSensor, touchSensor, keyboardSensor);

  if (isLoading) {
    return (
      <div className={`${masonryClasses} animate-fadeIn px-0 py-4 sm:py-6`}>
        {[...Array(6)].map((_, i) => (
          <div key={i} className="mb-4 break-inside-avoid sm:mb-6">
            <LoadingSkeleton />
          </div>
        ))}
      </div>
    );
  }

  if (!services || services.length === 0) {
    return (
      <div className="flex items-center justify-center h-[calc(100vh-12rem)] w-full">
        <div className="text-center p-8 rounded-lg bg-zinc-50 dark:bg-zinc-800/50 backdrop-blur-sm">
          <h3 className="text-xl font-medium text-zinc-900 dark:text-white mb-3">
            No Services Configured
          </h3>
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
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
          strategy={rectSortingStrategy}
        >
          <div className={masonryClasses}>
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
