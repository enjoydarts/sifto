FROM python:3.12-slim
WORKDIR /src
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
ENV PYTHONDONTWRITEBYTECODE=1
CMD ["python", "-B", "-m", "unittest", "discover", "-s", "tests", "-v"]
