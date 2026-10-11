FROM python:3.12-slim
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends openssh-client git && rm -rf /var/lib/apt/lists/*
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY maintenance ./maintenance
COPY tests ./tests
COPY playbooks ./playbooks
ENV PYTHONDONTWRITEBYTECODE=1
CMD ["python", "-B", "-m", "unittest", "discover", "-s", "tests", "-v"]
