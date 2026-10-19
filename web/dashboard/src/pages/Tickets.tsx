import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { ListResponse, Ticket } from "@/lib/types";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { Card, Modal, Spinner } from "@/components/ui/card";
import { EmptyState } from "@/components/EmptyState";
import { PageHeader } from "@/components/PageHeader";
import { cn, formatDateTime, priorityColor, timeAgo } from "@/lib/utils";

const page = copy.pages.tickets;

const COLUMNS = [
  { key: "open", label: page.columnKeys.open },
  { key: "in_progress", label: page.columnKeys.in_progress },
  { key: "waiting", label: page.columnKeys.waiting },
  { key: "resolved", label: page.columnKeys.resolved },
  { key: "closed", label: page.columnKeys.closed },
];

function priorityLabel(priority: string): string {
  const labels: Record<string, string> = { ...page.priorities };
  return labels[priority] || priority;
}

export function TicketsPage() {
  const qc = useQueryClient();
  const [editing, setEditing] = useState<Ticket | null>(null);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ title: "", description: "", priority: "normal", due_at: "" });
  const [dragging, setDragging] = useState<string | null>(null);
  const [note, setNote] = useState("");
  // Below md the five columns collapse into tabs; only one column is mounted.
  const [activeColumn, setActiveColumn] = useState(COLUMNS[0].key);

  const tickets = useQuery({
    queryKey: ["tickets"],
    queryFn: () => api<ListResponse<Ticket>>("/api/v1/tickets", { query: { status: "all", limit: 100 } }),
    refetchInterval: 30000,
  });

  const move = useMutation({
    mutationFn: (input: { id: string; status: string }) =>
      api(`/api/v1/tickets/${input.id}/status`, { method: "POST", body: { status: input.status } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tickets"] }),
  });

  const create = useMutation({
    mutationFn: () =>
      api("/api/v1/tickets", {
        method: "POST",
        body: {
          title: form.title,
          description: form.description,
          priority: form.priority,
          due_at: form.due_at ? new Date(form.due_at).toISOString() : "",
        },
      }),
    onSuccess: () => {
      setCreating(false);
      setForm({ title: "", description: "", priority: "normal", due_at: "" });
      qc.invalidateQueries({ queryKey: ["tickets"] });
    },
  });

  const update = useMutation({
    mutationFn: (ticket: Ticket) =>
      api(`/api/v1/tickets/${ticket.id}`, {
        method: "PUT",
        body: {
          title: ticket.title,
          description: ticket.description,
          priority: ticket.priority,
          due_at: ticket.due_at || "",
        },
      }),
    onSuccess: () => {
      setEditing(null);
      qc.invalidateQueries({ queryKey: ["tickets"] });
    },
  });

  const addNote = useMutation({
    mutationFn: (input: { id: string; text: string }) =>
      api(`/api/v1/tickets/${input.id}/notes`, { method: "POST", body: { text: input.text } }),
    onSuccess: () => {
      setNote("");
      qc.invalidateQueries({ queryKey: ["tickets"] });
    },
  });

  const items = tickets.data?.data || [];
  const byStatus = (status: string) => items.filter((x) => x.status === status);

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <PageHeader
        title={page.title}
        intro={page.intro}
        actions={
          <>
            <span className="chip bg-slate-100 text-slate-500">{items.length}</span>
            <Button onClick={() => setCreating(true)}>+ {t("newTicket")}</Button>
          </>
        }
      />

      {tickets.isLoading ? (
        <div className="flex justify-center py-10">
          <Spinner />
        </div>
      ) : items.length === 0 ? (
        <EmptyState title={page.empty.title} body={page.empty.body} cta={{ label: page.empty.cta, to: page.empty.to }} icon="🎫" />
      ) : (
        <>
          <div className="kanban-tabs">
            {COLUMNS.map((col) => (
              <button
                key={col.key}
                type="button"
                onClick={() => setActiveColumn(col.key)}
                className={cn(
                  "chip whitespace-nowrap border px-3 py-1.5",
                  activeColumn === col.key
                    ? "border-brand-200 bg-brand-50 text-brand-700"
                    : "border-slate-200 text-slate-500"
                )}
              >
                {col.label} · {byStatus(col.key).length}
              </button>
            ))}
          </div>
          <div className="kanban">
            {COLUMNS.map((col) => (
              <div
                key={col.key}
                data-column={col.key}
                data-active={activeColumn === col.key}
                onDragOver={(e) => e.preventDefault()}
                onDrop={() => {
                  if (dragging) move.mutate({ id: dragging, status: col.key });
                  setDragging(null);
                }}
                className="min-h-[300px] rounded-2xl bg-slate-100/70 p-2"
              >
                <div className="mb-2 flex items-center justify-between px-2 pt-1 text-xs font-semibold text-slate-500">
                  <span>{col.label}</span>
                  <span className="chip bg-white text-slate-500">{byStatus(col.key).length}</span>
                </div>
                <div className="space-y-2">
                  {byStatus(col.key).map((ticket) => (
                    <div
                      key={ticket.id}
                      draggable
                      onDragStart={() => setDragging(ticket.id)}
                      onClick={() => setEditing(ticket)}
                      className="cursor-pointer rounded-xl bg-white p-3 text-xs shadow-sm transition hover:shadow-md"
                    >
                      <div className="mb-1 flex items-center justify-between">
                        <span className={cn("chip px-1.5 py-0.5 text-[10px]", priorityColor(ticket.priority))}>
                          {priorityLabel(ticket.priority)}
                        </span>
                        {ticket.due_at && (
                          <span className="text-[10px] text-slate-400">{formatDateTime(ticket.due_at)}</span>
                        )}
                      </div>
                      <div className="font-medium text-slate-800">{ticket.title}</div>
                      {ticket.description && (
                        <div className="mt-1 line-clamp-2 text-[11px] text-slate-500">{ticket.description}</div>
                      )}
                      <div className="mt-2 flex items-center justify-between text-[10px] text-slate-400">
                        <span>{ticket.tags?.join(", ")}</span>
                        <span>{timeAgo(ticket.created_at)}</span>
                      </div>
                    </div>
                  ))}
                  {byStatus(col.key).length === 0 && (
                    <div className="py-8 text-center text-xs text-slate-400">{copy.misc.dash}</div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      <Modal
        open={creating}
        onClose={() => setCreating(false)}
        title={t("newTicket")}
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreating(false)}>
              {t("cancel")}
            </Button>
            <Button disabled={!form.title.trim() || create.isPending} onClick={() => create.mutate()}>
              {t("create")}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <Label>{t("title")}</Label>
            <Input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} />
          </div>
          <div>
            <Label>{t("description")}</Label>
            <Textarea value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>{t("priority")}</Label>
              <Select value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value })}>
                <option value="low">{page.priorities.low}</option>
                <option value="normal">{page.priorities.normal}</option>
                <option value="high">{page.priorities.high}</option>
                <option value="urgent">{page.priorities.urgent}</option>
              </Select>
            </div>
            <div>
              <Label>{t("dueDate")}</Label>
              <Input type="datetime-local" value={form.due_at} onChange={(e) => setForm({ ...form, due_at: e.target.value })} />
            </div>
          </div>
        </div>
      </Modal>

      <Modal
        open={!!editing}
        onClose={() => setEditing(null)}
        title={editing?.title || ""}
        footer={
          <>
            <Button variant="ghost" onClick={() => setEditing(null)}>
              {t("cancel")}
            </Button>
            <Button onClick={() => editing && update.mutate(editing)}>{t("save")}</Button>
          </>
        }
      >
        {editing && (
          <div className="space-y-3">
            <div>
              <Label>{t("title")}</Label>
              <Input value={editing.title} onChange={(e) => setEditing({ ...editing, title: e.target.value })} />
            </div>
            <div>
              <Label>{t("description")}</Label>
              <Textarea
                value={editing.description || ""}
                onChange={(e) => setEditing({ ...editing, description: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>{t("priority")}</Label>
                <Select
                  value={editing.priority}
                  onChange={(e) => setEditing({ ...editing, priority: e.target.value as Ticket["priority"] })}
                >
                  <option value="low">{page.priorities.low}</option>
                  <option value="normal">{page.priorities.normal}</option>
                  <option value="high">{page.priorities.high}</option>
                  <option value="urgent">{page.priorities.urgent}</option>
                </Select>
              </div>
              <div>
                <Label>{t("status")}</Label>
                <Select
                  value={editing.status}
                  onChange={(e) => {
                    move.mutate({ id: editing.id, status: e.target.value });
                    setEditing({ ...editing, status: e.target.value });
                  }}
                >
                  {COLUMNS.map((c) => (
                    <option key={c.key} value={c.key}>
                      {c.label}
                    </option>
                  ))}
                </Select>
              </div>
            </div>
            <div>
              <Label>{page.internalNotes}</Label>
              <div className="max-h-40 space-y-1 overflow-y-auto rounded-xl bg-slate-50 p-2 text-[11px]">
                {(editing.internal_notes || []).length === 0 && <span className="text-slate-400">{copy.misc.dash}</span>}
                {(editing.internal_notes || []).map((n) => (
                  <div key={n.id} className="rounded-lg bg-white p-2">
                    <div className="text-slate-700">{n.text}</div>
                    <div className="text-[9px] text-slate-400">{formatDateTime(n.at)}</div>
                  </div>
                ))}
              </div>
              <div className="mt-2 flex gap-2">
                <Input value={note} onChange={(e) => setNote(e.target.value)} placeholder={page.newNote} />
                <Button
                  size="sm"
                  disabled={!note.trim()}
                  onClick={async () => {
                    await addNote.mutateAsync({ id: editing.id, text: note });
                    const fresh = await api<Ticket>(`/api/v1/tickets/${editing.id}`);
                    setEditing(fresh);
                  }}
                >
                  {page.addNote}
                </Button>
              </div>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
