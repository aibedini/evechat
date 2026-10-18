import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { KBArticle, ListResponse } from "@/lib/types";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input, Label, Switch, Textarea } from "@/components/ui/input";
import { Card, Empty, Modal, Spinner } from "@/components/ui/card";
import { formatDateTime } from "@/lib/utils";

const blank = { id: "", title: "", content: "", tags: "", is_published: true };

export function KnowledgeBasePage() {
  const qc = useQueryClient();
  const [q, setQ] = useState("");
  const [editing, setEditing] = useState<typeof blank | null>(null);

  const list = useQuery({
    queryKey: ["kb", q],
    queryFn: () =>
      q
        ? api<ListResponse<KBArticle>>("/api/v1/knowledge-base/search", { query: { q, limit: 20 } })
        : api<ListResponse<KBArticle>>("/api/v1/knowledge-base"),
  });

  const save = useMutation({
    mutationFn: async (item: typeof blank) => {
      const body = {
        title: item.title,
        content: item.content,
        tags: item.tags.split(",").map((x) => x.trim()).filter(Boolean),
        is_published: item.is_published,
      };
      if (item.id) return api(`/api/v1/knowledge-base/${item.id}`, { method: "PUT", body });
      return api("/api/v1/knowledge-base", { method: "POST", body });
    },
    onSuccess: () => {
      setEditing(null);
      qc.invalidateQueries({ queryKey: ["kb"] });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/knowledge-base/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["kb"] }),
  });

  const items = list.data?.data || [];

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mb-4 flex items-center gap-3">
        <h1 className="text-lg font-bold">{t("knowledgeBase")}</h1>
        <input className="input ms-auto w-72" placeholder={t("searchKb")} value={q} onChange={(e) => setQ(e.target.value)} />
        <Button onClick={() => setEditing({ ...blank })}>+ {t("article")}</Button>
      </div>

      {list.isLoading ? (
        <div className="flex justify-center py-10">
          <Spinner />
        </div>
      ) : items.length === 0 ? (
        <Card>
          <Empty>{t("empty")}</Empty>
        </Card>
      ) : (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {items.map((a) => (
            <Card key={a.id} className="flex flex-col">
              <div className="mb-1 flex items-center gap-2">
                <h3 className="flex-1 truncate text-sm font-semibold">{a.title}</h3>
                <span className={`chip ${a.is_published ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500"}`}>
                  {a.is_published ? t("published") : t("draft")}
                </span>
              </div>
              <p className="line-clamp-3 flex-1 text-xs leading-6 text-slate-500">{a.content}</p>
              <div className="mt-2 flex items-center gap-2 text-[10px] text-slate-400">
                <span>{formatDateTime(a.updated_at)}</span>
                <span>· {a.view_count} بازدید</span>
                <div className="ms-auto flex gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      setEditing({
                        id: a.id,
                        title: a.title,
                        content: a.content,
                        tags: (a.tags || []).join(", "),
                        is_published: a.is_published,
                      })
                    }
                  >
                    {t("edit")}
                  </Button>
                  <Button
                    size="sm"
                    variant="danger"
                    onClick={() => confirm(t("confirmDelete")) && remove.mutate(a.id)}
                  >
                    {t("delete")}
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      <Modal
        open={!!editing}
        onClose={() => setEditing(null)}
        title={editing?.id ? t("edit") : t("article")}
        footer={
          <>
            <Button variant="ghost" onClick={() => setEditing(null)}>
              {t("cancel")}
            </Button>
            <Button disabled={!editing?.title.trim() || save.isPending} onClick={() => editing && save.mutate(editing)}>
              {t("save")}
            </Button>
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
              <Label>{t("content")}</Label>
              <Textarea
                className="min-h-[220px] font-mono text-xs leading-6"
                value={editing.content}
                onChange={(e) => setEditing({ ...editing, content: e.target.value })}
              />
            </div>
            <div>
              <Label>{t("tags")}</Label>
              <Input value={editing.tags} onChange={(e) => setEditing({ ...editing, tags: e.target.value })} placeholder="billing, plan" />
            </div>
            <Switch
              checked={editing.is_published}
              onChange={(v) => setEditing({ ...editing, is_published: v })}
              label={t("published")}
            />
          </div>
        )}
      </Modal>
    </div>
  );
}
