import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Agent, ListResponse } from "@/lib/types";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Switch } from "@/components/ui/input";
import { Card, Modal, Spinner } from "@/components/ui/card";
import { EmptyState } from "@/components/EmptyState";
import { FieldHelp } from "@/components/FieldHelp";
import { HelpTip } from "@/components/HelpTip";
import { PageHeader } from "@/components/PageHeader";
import { cn, timeAgo } from "@/lib/utils";

const page = copy.pages.team;

interface TeamForm {
  id: string;
  name: string;
  email: string;
  password: string;
  role: string;
  telegram_id: string;
  max_conversations: number;
  is_active: boolean;
}

const blank: TeamForm = {
  id: "",
  name: "",
  email: "",
  password: "",
  role: "agent",
  telegram_id: "",
  max_conversations: 10,
  is_active: true,
};

export function TeamSettingsPage() {
  const qc = useQueryClient();
  const [editing, setEditing] = useState<TeamForm | null>(null);
  const [reset, setReset] = useState<{ id: string; password: string } | null>(null);

  const team = useQuery({
    queryKey: ["agents"],
    queryFn: () => api<ListResponse<Agent>>("/api/v1/agents"),
  });

  const save = useMutation({
    mutationFn: (form: TeamForm) => {
      const body: any = {
        name: form.name,
        email: form.email,
        role: form.role,
        telegram_id: form.telegram_id || null,
        max_conversations: Number(form.max_conversations) || 10,
        is_active: form.is_active,
      };
      if (form.password) body.password = form.password;
      return form.id
        ? api(`/api/v1/agents/${form.id}`, { method: "PUT", body })
        : api("/api/v1/agents", { method: "POST", body });
    },
    onSuccess: () => {
      setEditing(null);
      qc.invalidateQueries({ queryKey: ["agents"] });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/agents/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["agents"] }),
  });

  const resetPassword = useMutation({
    mutationFn: (input: { id: string; password: string }) =>
      api(`/api/v1/agents/${input.id}/password`, { method: "PUT", body: { new_password: input.password } }),
    onSuccess: () => {
      setReset(null);
    },
  });

  const items = team.data?.data || [];

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <PageHeader
        title={page.title}
        intro={page.intro}
        guide={page.guide}
        actions={<Button onClick={() => setEditing({ ...blank })}>+ {page.addMember}</Button>}
      />

      <Card className="overflow-hidden p-0">
        {team.isLoading ? (
          <div className="flex justify-center py-8">
            <Spinner />
          </div>
        ) : items.length === 0 ? (
          <EmptyState
            title={page.empty.title}
            body={page.empty.body}
            cta={{ label: page.empty.cta, onClick: () => setEditing({ ...blank }) }}
            icon="👤"
          />
        ) : (
          <table className="table-cards w-full text-xs">
            <thead className="bg-slate-50 text-slate-500">
              <tr>
                <th className="px-4 py-3 text-start font-medium">{page.columns.name}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.email}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.role}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.telegram}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.max}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.status}</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {items.map((a) => (
                <tr key={a.id} className="border-t border-slate-50">
                  <td className="px-4 py-3 font-medium">{a.name}</td>
                  <td className="px-4 py-3" dir="ltr" data-label={page.columns.email}>
                    {a.email}
                  </td>
                  <td className="px-4 py-3">
                    <span className="chip bg-slate-100 text-slate-600">{page.roles[a.role as keyof typeof page.roles] || a.role}</span>
                  </td>
                  <td className="px-4 py-3" dir="ltr">
                    {a.telegram_id || copy.misc.dash}
                  </td>
                  <td className="px-4 py-3">{a.max_conversations}</td>
                  <td className="px-4 py-3">
                    <span className={cn("chip", a.is_online ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500")}>
                      {a.is_online ? t("online") : timeAgo(a.created_at)}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-end">
                    <div className="flex justify-end gap-1">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() =>
                          setEditing({
                            id: a.id,
                            name: a.name,
                            email: a.email,
                            password: "",
                            role: a.role,
                            telegram_id: a.telegram_id || "",
                            max_conversations: a.max_conversations,
                            is_active: a.is_active,
                          })
                        }
                      >
                        {t("edit")}
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => {
                          const pw = prompt(page.newPasswordPrompt);
                          if (pw && pw.length >= 8) resetPassword.mutate({ id: a.id, password: pw });
                        }}
                      >
                        {page.passwordButton}
                      </Button>
                      <Button
                        size="sm"
                        variant="danger"
                        onClick={() => confirm(t("confirmDelete")) && remove.mutate(a.id)}
                      >
                        {t("delete")}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>

      <Modal
        open={!!editing}
        onClose={() => setEditing(null)}
        title={editing?.id ? t("edit") : page.newMember}
        footer={
          <>
            <Button variant="ghost" onClick={() => setEditing(null)}>
              {t("cancel")}
            </Button>
            <Button
              disabled={!editing?.name.trim() || !editing?.email.trim() || save.isPending}
              onClick={() => editing && save.mutate(editing)}
            >
              {t("save")}
            </Button>
          </>
        }
      >
        {editing && (
          <div className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>{t("name")}</Label>
                <Input value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
              </div>
              <div>
                <Label>{t("email")}</Label>
                <Input dir="ltr" type="email" value={editing.email} onChange={(e) => setEditing({ ...editing, email: e.target.value })} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>
                  <span className="inline-flex items-center gap-1">
                    {t("role")}
                    <HelpTip text={page.rolesHelp} />
                  </span>
                </Label>
                <Select value={editing.role} onChange={(e) => setEditing({ ...editing, role: e.target.value })}>
                  <option value="agent">{page.roles.agent}</option>
                  <option value="admin">{page.roles.admin}</option>
                  <option value="superadmin">{page.roles.superadmin}</option>
                </Select>
              </div>
              <div>
                <Label>{t("telegramId")}</Label>
                <Input dir="ltr" value={editing.telegram_id} onChange={(e) => setEditing({ ...editing, telegram_id: e.target.value })} />
                <FieldHelp>{page.telegramHelp}</FieldHelp>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>{t("maxConversations")}</Label>
                <Input
                  type="number"
                  value={editing.max_conversations}
                  onChange={(e) => setEditing({ ...editing, max_conversations: Number(e.target.value) })}
                />
                <FieldHelp>{page.maxHelp}</FieldHelp>
              </div>
              <div>
                <Label>{editing.id ? page.newPasswordOptional : t("password")}</Label>
                <Input
                  dir="ltr"
                  type="password"
                  value={editing.password}
                  onChange={(e) => setEditing({ ...editing, password: e.target.value })}
                />
              </div>
            </div>
            <Switch checked={editing.is_active} onChange={(v) => setEditing({ ...editing, is_active: v })} label={page.active} />
          </div>
        )}
      </Modal>
      <span className="hidden">{reset ? "reset" : ""}</span>
    </div>
  );
}
