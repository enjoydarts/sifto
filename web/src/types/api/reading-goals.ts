export interface ReadingGoal {
  id: string;
  user_id: string;
  title: string;
  description: string;
  priority: number;
  status: "active" | "archived" | string;
  due_date?: string | null;
  created_at?: string;
  updated_at?: string;
}
